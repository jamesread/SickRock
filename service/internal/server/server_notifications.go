package server

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	sickrockpb "github.com/jamesread/SickRock/gen/proto"
	repo "github.com/jamesread/SickRock/internal/repo"
)

// GetNotificationEvents retrieves all available notification events
func (s *SickRockServer) GetNotificationEvents(ctx context.Context, req *connect.Request[sickrockpb.GetNotificationEventsRequest]) (*connect.Response[sickrockpb.GetNotificationEventsResponse], error) {
	events, err := s.repo.GetNotificationEvents(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve notification events: %w", err))
	}

	var pbEvents []*sickrockpb.NotificationEvent
	for _, event := range events {
		pbEvents = append(pbEvents, &sickrockpb.NotificationEvent{
			Id:          toInt32(event.ID),
			EventCode:   event.EventCode,
			EventName:   event.EventName,
			Description: event.Description,
		})
	}

	return connect.NewResponse(&sickrockpb.GetNotificationEventsResponse{
		Events: pbEvents,
	}), nil
}

// GetUserNotificationChannels retrieves all notification channels for the authenticated user
func (s *SickRockServer) GetUserNotificationChannels(ctx context.Context, req *connect.Request[sickrockpb.GetUserNotificationChannelsRequest]) (*connect.Response[sickrockpb.GetUserNotificationChannelsResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	channels, err := s.repo.GetUserNotificationChannels(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve notification channels: %w", err))
	}

	return connect.NewResponse(&sickrockpb.GetUserNotificationChannelsResponse{
		Channels: notificationChannelsToProto(channels),
	}), nil
}

func notificationChannelsToProto(channels []repo.UserNotificationChannel) []*sickrockpb.UserNotificationChannel {
	var pbChannels []*sickrockpb.UserNotificationChannel
	for _, channel := range channels {
		pbChannels = append(pbChannels, notificationChannelToProto(channel))
	}
	return pbChannels
}

func notificationChannelToProto(channel repo.UserNotificationChannel) *sickrockpb.UserNotificationChannel {
	pbChannel := &sickrockpb.UserNotificationChannel{
		Id:           toInt32(channel.ID),
		UserId:       toInt32(channel.User),
		ChannelType:  channel.ChannelType,
		ChannelValue: channel.ChannelValue,
		IsActive:     channel.IsActive,
		SrCreated:    channel.SrCreated.Unix(),
		SrUpdated:    channel.SrUpdated.Unix(),
	}
	if channel.ChannelName != nil {
		pbChannel.ChannelName = *channel.ChannelName
	}
	return pbChannel
}

// CreateUserNotificationChannel creates a new notification channel for the authenticated user
func (s *SickRockServer) CreateUserNotificationChannel(ctx context.Context, req *connect.Request[sickrockpb.CreateUserNotificationChannelRequest]) (*connect.Response[sickrockpb.CreateUserNotificationChannelResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	channelType := strings.TrimSpace(strings.ToLower(req.Msg.GetChannelType()))
	channelValue := strings.TrimSpace(req.Msg.GetChannelValue())

	if !slices.Contains([]string{"email", "telegram", "webhook"}, channelType) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid channel type: must be 'email', 'telegram', or 'webhook'"))
	}
	if err = validateChannelValue(channelType, channelValue); err != nil {
		return nil, err
	}

	channel, err := s.repo.CreateUserNotificationChannel(ctx, userID, channelType, channelValue, optionalString(req.Msg.GetChannelName()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create notification channel: %w", err))
	}

	return connect.NewResponse(&sickrockpb.CreateUserNotificationChannelResponse{
		Success: true,
		Message: "Notification channel created successfully",
		Channel: notificationChannelToProto(*channel),
	}), nil
}

// validateChannelValue checks a channel value is present and, for email and webhook channels, plausibly formed.
func validateChannelValue(channelType, channelValue string) error {
	if channelValue == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel value is required"))
	}
	if channelType == "email" && !strings.Contains(channelValue, "@") {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid email address"))
	}
	if channelType == "webhook" && !strings.HasPrefix(channelValue, "http://") && !strings.HasPrefix(channelValue, "https://") {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("webhook URL must start with http:// or https://"))
	}
	return nil
}

// optionalString trims value, returning nil when it is empty.
func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// UpdateUserNotificationChannel updates a notification channel
func (s *SickRockServer) UpdateUserNotificationChannel(ctx context.Context, req *connect.Request[sickrockpb.UpdateUserNotificationChannelRequest]) (*connect.Response[sickrockpb.UpdateUserNotificationChannelResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	channelID := int(req.Msg.GetChannelId())
	if channelID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel ID is required"))
	}

	channel, err := s.ownedUserNotificationChannel(ctx, userID, channelID)
	if err != nil {
		return nil, err
	}

	channelValue := strings.TrimSpace(req.Msg.GetChannelValue())
	if err = validateChannelValue(channel.ChannelType, channelValue); err != nil {
		return nil, err
	}

	err = s.repo.UpdateUserNotificationChannel(ctx, channelID, channelValue, optionalString(req.Msg.GetChannelName()), req.Msg.GetIsActive())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update notification channel: %w", err))
	}

	return connect.NewResponse(&sickrockpb.UpdateUserNotificationChannelResponse{
		Success: true,
		Message: "Notification channel updated successfully",
	}), nil
}

// DeleteUserNotificationChannel deletes a notification channel
func (s *SickRockServer) DeleteUserNotificationChannel(ctx context.Context, req *connect.Request[sickrockpb.DeleteUserNotificationChannelRequest]) (*connect.Response[sickrockpb.DeleteUserNotificationChannelResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	channelID := int(req.Msg.GetChannelId())
	if channelID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel ID is required"))
	}

	// Verify the channel belongs to the user
	channel, err := s.repo.GetUserNotificationChannelByID(ctx, channelID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve channel: %w", err))
	}
	if channel == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("channel not found"))
	}
	if channel.User != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("channel does not belong to user"))
	}

	err = s.repo.DeleteUserNotificationChannel(ctx, channelID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete notification channel: %w", err))
	}

	return connect.NewResponse(&sickrockpb.DeleteUserNotificationChannelResponse{
		Success: true,
		Message: "Notification channel deleted successfully",
	}), nil
}

// GetUserNotificationSubscriptions retrieves all notification subscriptions for the authenticated user
func (s *SickRockServer) GetUserNotificationSubscriptions(ctx context.Context, req *connect.Request[sickrockpb.GetUserNotificationSubscriptionsRequest]) (*connect.Response[sickrockpb.GetUserNotificationSubscriptionsResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	subscriptions, err := s.repo.GetUserNotificationSubscriptions(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve notification subscriptions: %w", err))
	}

	// Get all events and channels for enrichment
	events, err := s.repo.GetNotificationEvents(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve events: %w", err))
	}
	channels, err := s.repo.GetUserNotificationChannels(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve channels: %w", err))
	}

	return connect.NewResponse(&sickrockpb.GetUserNotificationSubscriptionsResponse{
		Subscriptions: notificationSubscriptionsToProto(subscriptions, events, channels),
	}), nil
}

// notificationSubscriptionsToProto converts subscriptions, attaching their event and channel when known.
func notificationSubscriptionsToProto(subscriptions []repo.UserNotificationSubscription, events []repo.NotificationEvent, channels []repo.UserNotificationChannel) []*sickrockpb.UserNotificationSubscription {
	eventMap := make(map[int]*sickrockpb.NotificationEvent, len(events))
	for i := range events {
		eventMap[events[i].ID] = notificationEventToProto(&events[i])
	}
	channelMap := make(map[int]*sickrockpb.UserNotificationChannel, len(channels))
	for _, channel := range channels {
		channelMap[channel.ID] = notificationChannelToProto(channel)
	}

	var pbSubscriptions []*sickrockpb.UserNotificationSubscription
	for _, sub := range subscriptions {
		pbSubscriptions = append(pbSubscriptions, &sickrockpb.UserNotificationSubscription{
			Id:        toInt32(sub.ID),
			UserId:    toInt32(sub.User),
			EventId:   toInt32(sub.EventID),
			ChannelId: toInt32(sub.ChannelID),
			SrCreated: sub.SrCreated.Unix(),
			Event:     eventMap[sub.EventID],
			Channel:   channelMap[sub.ChannelID],
		})
	}
	return pbSubscriptions
}

func notificationEventToProto(event *repo.NotificationEvent) *sickrockpb.NotificationEvent {
	return &sickrockpb.NotificationEvent{
		Id:          toInt32(event.ID),
		EventCode:   event.EventCode,
		EventName:   event.EventName,
		Description: event.Description,
	}
}

// CreateUserNotificationSubscription creates a new notification subscription
func (s *SickRockServer) CreateUserNotificationSubscription(ctx context.Context, req *connect.Request[sickrockpb.CreateUserNotificationSubscriptionRequest]) (*connect.Response[sickrockpb.CreateUserNotificationSubscriptionResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	eventCode := strings.TrimSpace(req.Msg.GetEventCode())
	channelID := int(req.Msg.GetChannelId())

	if err = validateSubscriptionRequest(eventCode, channelID); err != nil {
		return nil, err
	}

	event, err := s.notificationEventByCode(ctx, eventCode)
	if err != nil {
		return nil, err
	}

	channel, err := s.activeUserNotificationChannel(ctx, userID, channelID)
	if err != nil {
		return nil, err
	}

	if err = s.requireNoNotificationSubscription(ctx, userID, event.ID, channelID); err != nil {
		return nil, err
	}

	subscription, err := s.repo.CreateUserNotificationSubscription(ctx, userID, event.ID, channelID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create subscription: %w", err))
	}

	pbSub := &sickrockpb.UserNotificationSubscription{
		Id:        toInt32(subscription.ID),
		UserId:    toInt32(subscription.User),
		EventId:   toInt32(subscription.EventID),
		ChannelId: toInt32(subscription.ChannelID),
		SrCreated: subscription.SrCreated.Unix(),
		Event:     notificationEventToProto(event),
		Channel:   notificationChannelToProto(*channel),
	}

	return connect.NewResponse(&sickrockpb.CreateUserNotificationSubscriptionResponse{
		Success:      true,
		Message:      "Notification subscription created successfully",
		Subscription: pbSub,
	}), nil
}

// activeUserNotificationChannel loads a channel, verifying it belongs to the user and is active.
func (s *SickRockServer) activeUserNotificationChannel(ctx context.Context, userID, channelID int) (*repo.UserNotificationChannel, error) {
	channel, err := s.ownedUserNotificationChannel(ctx, userID, channelID)
	if err != nil {
		return nil, err
	}
	if !channel.IsActive {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel is not active"))
	}
	return channel, nil
}

// ownedUserNotificationChannel loads a channel, verifying it belongs to the user.
func (s *SickRockServer) ownedUserNotificationChannel(ctx context.Context, userID, channelID int) (*repo.UserNotificationChannel, error) {
	channel, err := s.repo.GetUserNotificationChannelByID(ctx, channelID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve channel: %w", err))
	}
	if channel == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("channel not found"))
	}
	if channel.User != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("channel does not belong to user"))
	}
	return channel, nil
}

// requireNoNotificationSubscription fails with AlreadyExists when the user already subscribes the channel to the event.
func (s *SickRockServer) requireNoNotificationSubscription(ctx context.Context, userID, eventID, channelID int) error {
	existingSubs, err := s.repo.GetUserNotificationSubscriptions(ctx, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve subscriptions: %w", err))
	}
	for _, sub := range existingSubs {
		if sub.EventID == eventID && sub.ChannelID == channelID {
			return connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("subscription already exists"))
		}
	}
	return nil
}

// DeleteUserNotificationSubscription deletes a notification subscription
func (s *SickRockServer) DeleteUserNotificationSubscription(ctx context.Context, req *connect.Request[sickrockpb.DeleteUserNotificationSubscriptionRequest]) (*connect.Response[sickrockpb.DeleteUserNotificationSubscriptionResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	subscriptionID := int(req.Msg.GetSubscriptionId())
	if subscriptionID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("subscription ID is required"))
	}

	// Verify the subscription belongs to the user
	subscription, err := s.repo.GetUserNotificationSubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve subscription: %w", err))
	}
	if subscription == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("subscription not found"))
	}
	if subscription.User != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("subscription does not belong to user"))
	}

	err = s.repo.DeleteUserNotificationSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete subscription: %w", err))
	}

	return connect.NewResponse(&sickrockpb.DeleteUserNotificationSubscriptionResponse{
		Success: true,
		Message: "Notification subscription deleted successfully",
	}), nil
}

func validateSubscriptionRequest(eventCode string, channelID int) error {
	if eventCode == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("event code is required"))
	}
	if channelID <= 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel ID is required"))
	}
	return nil
}

// notificationEventByCode loads a notification event, failing with NotFound when there is none.
func (s *SickRockServer) notificationEventByCode(ctx context.Context, eventCode string) (*repo.NotificationEvent, error) {
	event, err := s.repo.GetNotificationEventByCode(ctx, eventCode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve event: %w", err))
	}
	if event == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("event not found: %s", eventCode))
	}
	return event, nil
}
