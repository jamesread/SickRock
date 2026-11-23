package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"connectrpc.com/connect"
	log "github.com/sirupsen/logrus"

	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	"github.com/jamesread/SickRock/internal/iam/systemroles"
	"github.com/jamesread/SickRock/internal/notifications"
)

// Limits on user-to-user notifications (NotifyUsers).
const (
	NotifyMaxRecipients    = 20
	NotifyMaxTitleLength   = 200
	NotifyMaxMessageLength = 2000
	NotifyMaxLinkLength    = 1000
	// NotifyRateLimit recipients may be notified per sender in NotifyRateWindow.
	NotifyRateLimit   = 30
	NotifyRateWindow  = time.Hour
	notifySendTimeout = 20 * time.Second
)

// Statuses in NotifyUserResult.
const (
	NotifyStatusSent          = "sent"
	NotifyStatusNotSubscribed = "not_subscribed"
	NotifyStatusFailed        = "failed"
)

// notifyLimiter counts recent notifications per sender, in memory: at most limit in each window
// (NotifyRateLimit per NotifyRateWindow when unset).
type notifyLimiter struct {
	sent   map[int][]time.Time
	now    func() time.Time
	limit  int
	window time.Duration
	mu     sync.Mutex
}

// allow records n notifications by sender and reports whether they fit in the rate limit.
func (l *notifyLimiter) allow(sender, n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	if l.sent == nil {
		l.sent = map[int][]time.Time{}
	}
	limit, window := l.limit, l.window
	if limit <= 0 {
		limit, window = NotifyRateLimit, NotifyRateWindow
	}
	recent := l.prune(sender, now().Add(-window))
	if len(recent)+n > limit {
		l.sent[sender] = recent
		return false
	}
	for range n {
		recent = append(recent, now())
	}
	l.sent[sender] = recent
	return true
}

func (l *notifyLimiter) clock() func() time.Time {
	if l.now != nil {
		return l.now
	}
	return time.Now
}

func (l *notifyLimiter) prune(sender int, cutoff time.Time) []time.Time {
	recent := l.sent[sender][:0]
	for _, at := range l.sent[sender] {
		if at.After(cutoff) {
			recent = append(recent, at)
		}
	}
	return recent
}

// requireNotificationsSend checks the caller may send notifications. Connect calls are also
// gated by procedurePermissions; MCP tools call the server directly, so it is checked here too.
func (s *SickRockServer) requireNotificationsSend(ctx context.Context) error {
	au := s.authUserRBAC(ctx)
	if !isAuthenticated(au) {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if isSuperuser(au) || (au.RBAC != nil && au.RBAC.Has(systemroles.PermissionNotificationsSend)) {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("missing permission %s", systemroles.PermissionNotificationsSend))
}

// ListNotifiableUsers lists the users the caller may notify: those sharing an IAM group with them.
func (s *SickRockServer) ListNotifiableUsers(ctx context.Context, _ *connect.Request[sickrockpb.ListNotifiableUsersRequest]) (*connect.Response[sickrockpb.ListNotifiableUsersResponse], error) {
	if err := s.requireNotificationsSend(ctx); err != nil {
		return nil, err
	}
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.repo.ListNotifiableUsers(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list users: %w", err))
	}
	out := make([]*sickrockpb.NotifiableUser, 0, len(users))
	for _, u := range users {
		out = append(out, &sickrockpb.NotifiableUser{Id: toInt32(u.ID), Username: u.Username})
	}
	return connect.NewResponse(&sickrockpb.ListNotifiableUsersResponse{Users: out}), nil
}

// validNotifyLinkPath reports whether path is a path on this site: it starts with one "/" (so
// it cannot name another host) and holds no spaces, backslashes or control characters.
func validNotifyLinkPath(path string) bool {
	if path == "" {
		return true
	}
	if len(path) > NotifyMaxLinkLength || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return false
	}
	return !strings.ContainsFunc(path, notifyLinkBadRune)
}

func notifyLinkBadRune(r rune) bool {
	return r == '\\' || unicode.IsSpace(r) || unicode.IsControl(r)
}

func hasControlCharacters(s string, allowNewlines bool) bool {
	for _, r := range s {
		if isDisallowedControl(r, allowNewlines) {
			return true
		}
	}
	return false
}

func isDisallowedControl(r rune, allowNewlines bool) bool {
	if !unicode.IsControl(r) {
		return false
	}
	if !allowNewlines {
		return true
	}
	return r != '\n' && r != '\r' && r != '\t'
}

type notifyRequest struct {
	title      string
	message    string
	linkPath   string
	recipients []int
}

func invalidNotify(format string, args ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, args...))
}

// notifyRecipients returns the requested recipients once each, refusing bad ids and the sender.
func notifyRecipients(ids []int32, senderID int) ([]int, error) {
	seen := map[int]bool{}
	var recipients []int
	for _, id := range ids {
		recip, err := notifyRecipientID(id, senderID, seen)
		if err != nil {
			return nil, err
		}
		if recip > 0 {
			recipients = append(recipients, recip)
		}
	}
	return checkRecipientCount(recipients)
}

func notifyRecipientID(id int32, senderID int, seen map[int]bool) (int, error) {
	if id <= 0 {
		return 0, invalidNotify("invalid recipient")
	}
	if int(id) == senderID {
		return 0, invalidNotify("you cannot notify yourself")
	}
	if seen[int(id)] {
		return 0, nil
	}
	seen[int(id)] = true
	return int(id), nil
}

func checkRecipientCount(recipients []int) ([]int, error) {
	if len(recipients) == 0 {
		return nil, invalidNotify("choose at least one recipient")
	}
	if len(recipients) > NotifyMaxRecipients {
		return nil, invalidNotify("at most %d recipients", NotifyMaxRecipients)
	}
	return recipients, nil
}

// notifyTextProblem checks the title, message and link of a notification.
func notifyTextProblem(req *notifyRequest) error {
	if err := notifyTitleProblem(req.title); err != nil {
		return err
	}
	if err := notifyMessageProblem(req.message); err != nil {
		return err
	}
	if !validNotifyLinkPath(req.linkPath) {
		return invalidNotify("the link must be a path on this site, starting with /")
	}
	return nil
}

func notifyTitleProblem(title string) error {
	switch {
	case title == "":
		return invalidNotify("a title is required")
	case utf8.RuneCountInString(title) > NotifyMaxTitleLength:
		return invalidNotify("the title is longer than %d characters", NotifyMaxTitleLength)
	case hasControlCharacters(title, false):
		return invalidNotify("the title has control characters")
	}
	return nil
}

func notifyMessageProblem(message string) error {
	switch {
	case utf8.RuneCountInString(message) > NotifyMaxMessageLength:
		return invalidNotify("the message is longer than %d characters", NotifyMaxMessageLength)
	case hasControlCharacters(message, true):
		return invalidNotify("the message has control characters")
	}
	return nil
}

// validateNotifyRequest trims and checks a NotifyUsers request, dropping repeated recipients.
func validateNotifyRequest(msg *sickrockpb.NotifyUsersRequest, senderID int) (*notifyRequest, error) {
	recipients, err := notifyRecipients(msg.GetRecipientUserIds(), senderID)
	if err != nil {
		return nil, err
	}
	req := &notifyRequest{
		title:      strings.TrimSpace(msg.GetTitle()),
		message:    strings.TrimSpace(msg.GetMessage()),
		linkPath:   strings.TrimSpace(msg.GetLinkPath()),
		recipients: recipients,
	}
	if err := notifyTextProblem(req); err != nil {
		return nil, err
	}
	return req, nil
}

// notifiableUsernames returns the usernames of recipients, failing unless the sender may
// notify every one of them.
func (s *SickRockServer) notifiableUsernames(ctx context.Context, senderID int, recipients []int) (map[int]string, error) {
	allowed, err := s.repo.ListNotifiableUsers(ctx, senderID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list users: %w", err))
	}
	usernames := make(map[int]string, len(allowed))
	for _, u := range allowed {
		usernames[u.ID] = u.Username
	}
	for _, id := range recipients {
		if _, ok := usernames[id]; !ok {
			// The same answer whether or not the user exists.
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you can only notify users who share a group with you"))
		}
	}
	return usernames, nil
}

// NotifyUsers sends a user.notified notification from the caller to each recipient, on the
// recipient's channels subscribed to it, and reports what happened for each.
func (s *SickRockServer) NotifyUsers(ctx context.Context, req *connect.Request[sickrockpb.NotifyUsersRequest]) (*connect.Response[sickrockpb.NotifyUsersResponse], error) {
	if err := s.requireNotificationsSend(ctx); err != nil {
		return nil, err
	}
	au := s.authUser(ctx)
	nr, err := validateNotifyRequest(req.Msg, au.User.ID)
	if err != nil {
		return nil, err
	}
	usernames, err := s.notifiableUsernames(ctx, au.User.ID, nr.recipients)
	if err != nil {
		return nil, err
	}
	if !s.notifyLimiter.allow(au.User.ID, len(nr.recipients)) {
		return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("too many notifications: at most %d an hour", NotifyRateLimit))
	}
	results := s.notifyAll(ctx, nr.recipients, usernames, notifyEventData(ctx, au.User.Username, au.User.ID, nr))
	logNotifyResults(au.User.Username, nr.title, results)
	return connect.NewResponse(&sickrockpb.NotifyUsersResponse{Results: results}), nil
}

func notifyEventData(ctx context.Context, sender string, senderID int, nr *notifyRequest) map[string]any {
	data := map[string]any{
		"sender":    sender,
		"sender_id": senderID,
		"title":     nr.title,
		"message":   nr.message,
	}
	if nr.linkPath != "" {
		data["link"] = RequestOrigin(ctx) + nr.linkPath
	}
	return data
}

func logNotifyResults(sender, title string, results []*sickrockpb.NotifyUserResult) {
	fields := log.Fields{"sender": sender, "title": title}
	for _, r := range results {
		fields["to_"+r.GetUsername()] = r.GetStatus()
	}
	log.WithFields(fields).Info("User notification sent")
}

// notifyAll sends to every recipient in parallel. Sending outlives the request (a client that
// goes away does not cut it short) but is bounded by notifySendTimeout.
func (s *SickRockServer) notifyAll(ctx context.Context, recipients []int, usernames map[int]string, data map[string]any) []*sickrockpb.NotifyUserResult {
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notifySendTimeout)
	defer cancel()
	results := make([]*sickrockpb.NotifyUserResult, len(recipients))
	var wg sync.WaitGroup
	for i, id := range recipients {
		wg.Go(func() {
			results[i] = s.notifyOneUser(sendCtx, id, usernames[id], data)
		})
	}
	wg.Wait()
	return results
}

func (s *SickRockServer) notifyOneUser(ctx context.Context, userID int, username string, data map[string]any) *sickrockpb.NotifyUserResult {
	result := &sickrockpb.NotifyUserResult{UserId: toInt32(userID), Username: username}
	subscribed, delivered, err := s.notificationService.SendToUser(ctx, userID, notifications.EventUserNotified, data)
	result.DeliveredChannels = toInt32(delivered)
	switch {
	case err != nil:
		log.WithError(err).WithField("user_id", userID).Warn("Could not notify user")
		result.Status = NotifyStatusFailed
	case subscribed == 0:
		result.Status = NotifyStatusNotSubscribed
	case delivered == 0:
		result.Status = NotifyStatusFailed
	default:
		result.Status = NotifyStatusSent
	}
	return result
}

type requestOriginKey struct{}

// WithRequestOrigin records the scheme and host the request was made to (honouring a reverse
// proxy's X-Forwarded-Proto and X-Forwarded-Host), for links back to this site.
func WithRequestOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin, ok := requestOrigin(r); ok {
			r = r.WithContext(context.WithValue(r.Context(), requestOriginKey{}, origin))
		}
		next.ServeHTTP(w, r)
	})
}

func requestOrigin(r *http.Request) (string, bool) {
	host := requestHost(r)
	if host == "" || strings.ContainsAny(host, "/\\@ ") {
		return "", false
	}
	return requestScheme(r) + "://" + host, true
}

func requestScheme(r *http.Request) string {
	if proto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); proto == "http" || proto == "https" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(r *http.Request) string {
	if forwarded := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		return forwarded
	}
	return r.Host
}

func firstHeaderValue(value string) string {
	first, _, _ := strings.Cut(value, ",")
	return strings.TrimSpace(first)
}

// RequestOrigin returns the origin recorded by WithRequestOrigin, or "" (leaving links as paths).
func RequestOrigin(ctx context.Context) string {
	origin, _ := ctx.Value(requestOriginKey{}).(string)
	return origin
}
