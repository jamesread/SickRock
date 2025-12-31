package server

import (
	"context"
	"fmt"
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
			Id:          int32(event.ID),
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

	var pbChannels []*sickrockpb.UserNotificationChannel
	for _, channel := range channels {
		pbChannel := &sickrockpb.UserNotificationChannel{
			Id:          int32(channel.ID),
			UserId:      int32(channel.User),
			ChannelType: channel.ChannelType,
			ChannelValue: channel.ChannelValue,
			IsActive:    channel.IsActive,
			SrCreated:   channel.SrCreated.Unix(),
			SrUpdated:   channel.SrUpdated.Unix(),
		}
		if channel.ChannelName != nil {
			pbChannel.ChannelName = *channel.ChannelName
		}
		pbChannels = append(pbChannels, pbChannel)
	}

	return connect.NewResponse(&sickrockpb.GetUserNotificationChannelsResponse{
		Channels: pbChannels,
	}), nil
}

// CreateUserNotificationChannel creates a new notification channel for the authenticated user
func (s *SickRockServer) CreateUserNotificationChannel(ctx context.Context, req *connect.Request[sickrockpb.CreateUserNotificationChannelRequest]) (*connect.Response[sickrockpb.CreateUserNotificationChannelResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	channelType := strings.TrimSpace(strings.ToLower(req.Msg.GetChannelType()))
	channelValue := strings.TrimSpace(req.Msg.GetChannelValue())
	channelName := strings.TrimSpace(req.Msg.GetChannelName())

	// Validate channel type
	if channelType != "email" && channelType != "telegram" && channelType != "webhook" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid channel type: must be 'email', 'telegram', or 'webhook'"))
	}

	// Validate channel value
	if channelValue == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel value is required"))
	}

	// Basic validation for email
	if channelType == "email" {
		if !strings.Contains(channelValue, "@") {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid email address"))
		}
	}

	// Basic validation for webhook URL
	if channelType == "webhook" {
		if !strings.HasPrefix(channelValue, "http://") && !strings.HasPrefix(channelValue, "https://") {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("webhook URL must start with http:// or https://"))
		}
	}

	var channelNamePtr *string
	if channelName != "" {
		channelNamePtr = &channelName
	}

	channel, err := s.repo.CreateUserNotificationChannel(ctx, userID, channelType, channelValue, channelNamePtr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create notification channel: %w", err))
	}

	pbChannel := &sickrockpb.UserNotificationChannel{
		Id:          int32(channel.ID),
		UserId:      int32(channel.User),
		ChannelType: channel.ChannelType,
		ChannelValue: channel.ChannelValue,
		IsActive:    channel.IsActive,
		SrCreated:   channel.SrCreated.Unix(),
		SrUpdated:   channel.SrUpdated.Unix(),
	}
	if channel.ChannelName != nil {
		pbChannel.ChannelName = *channel.ChannelName
	}

	return connect.NewResponse(&sickrockpb.CreateUserNotificationChannelResponse{
		Success: true,
		Message: "Notification channel created successfully",
		Channel: pbChannel,
	}), nil
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

	channelValue := strings.TrimSpace(req.Msg.GetChannelValue())
	channelName := strings.TrimSpace(req.Msg.GetChannelName())
	isActive := req.Msg.GetIsActive()

	if channelValue == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel value is required"))
	}

	// Basic validation based on channel type
	if channel.ChannelType == "email" {
		if !strings.Contains(channelValue, "@") {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid email address"))
		}
	}
	if channel.ChannelType == "webhook" {
		if !strings.HasPrefix(channelValue, "http://") && !strings.HasPrefix(channelValue, "https://") {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("webhook URL must start with http:// or https://"))
		}
	}

	var channelNamePtr *string
	if channelName != "" {
		channelNamePtr = &channelName
	}

	err = s.repo.UpdateUserNotificationChannel(ctx, channelID, channelValue, channelNamePtr, isActive)
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
	eventMap := make(map[int]*repo.NotificationEvent)
	for i := range events {
		eventMap[events[i].ID] = &events[i]
	}

	channels, err := s.repo.GetUserNotificationChannels(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve channels: %w", err))
	}
	channelMap := make(map[int]*repo.UserNotificationChannel)
	for i := range channels {
		channelMap[channels[i].ID] = &channels[i]
	}

	var pbSubscriptions []*sickrockpb.UserNotificationSubscription
	for _, sub := range subscriptions {
		pbSub := &sickrockpb.UserNotificationSubscription{
			Id:        int32(sub.ID),
			UserId:    int32(sub.User),
			EventId:   int32(sub.EventID),
			ChannelId: int32(sub.ChannelID),
			SrCreated: sub.SrCreated.Unix(),
		}

		// Add event details
		if event, ok := eventMap[sub.EventID]; ok {
			pbSub.Event = &sickrockpb.NotificationEvent{
				Id:          int32(event.ID),
				EventCode:   event.EventCode,
				EventName:   event.EventName,
				Description: event.Description,
			}
		}

		// Add channel details
		if channel, ok := channelMap[sub.ChannelID]; ok {
			pbChannel := &sickrockpb.UserNotificationChannel{
				Id:          int32(channel.ID),
				UserId:      int32(channel.User),
				ChannelType: channel.ChannelType,
				ChannelValue: channel.ChannelValue,
				IsActive:    channel.IsActive,
				SrCreated:   channel.SrCreated.Unix(),
				SrUpdated:   channel.SrUpdated.Unix(),
			}
			if channel.ChannelName != nil {
				pbChannel.ChannelName = *channel.ChannelName
			}
			pbSub.Channel = pbChannel
		}

		pbSubscriptions = append(pbSubscriptions, pbSub)
	}

	return connect.NewResponse(&sickrockpb.GetUserNotificationSubscriptionsResponse{
		Subscriptions: pbSubscriptions,
	}), nil
}

// CreateUserNotificationSubscription creates a new notification subscription
func (s *SickRockServer) CreateUserNotificationSubscription(ctx context.Context, req *connect.Request[sickrockpb.CreateUserNotificationSubscriptionRequest]) (*connect.Response[sickrockpb.CreateUserNotificationSubscriptionResponse], error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	eventCode := strings.TrimSpace(req.Msg.GetEventCode())
	channelID := int(req.Msg.GetChannelId())

	if eventCode == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("event code is required"))
	}
	if channelID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel ID is required"))
	}

	// Get event by code
	event, err := s.repo.GetNotificationEventByCode(ctx, eventCode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve event: %w", err))
	}
	if event == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("event not found: %s", eventCode))
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
	if !channel.IsActive {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel is not active"))
	}

	// Check if subscription already exists
	existingSubs, err := s.repo.GetUserNotificationSubscriptions(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve subscriptions: %w", err))
	}
	for _, sub := range existingSubs {
		if sub.EventID == event.ID && sub.ChannelID == channelID {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("subscription already exists"))
		}
	}

	subscription, err := s.repo.CreateUserNotificationSubscription(ctx, userID, event.ID, channelID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create subscription: %w", err))
	}

	pbSub := &sickrockpb.UserNotificationSubscription{
		Id:        int32(subscription.ID),
		UserId:    int32(subscription.User),
		EventId:   int32(subscription.EventID),
		ChannelId: int32(subscription.ChannelID),
		SrCreated: subscription.SrCreated.Unix(),
	}

	// Add event and channel details
	pbSub.Event = &sickrockpb.NotificationEvent{
		Id:          int32(event.ID),
		EventCode:   event.EventCode,
		EventName:   event.EventName,
		Description: event.Description,
	}

	pbChannel := &sickrockpb.UserNotificationChannel{
		Id:          int32(channel.ID),
		UserId:      int32(channel.User),
		ChannelType: channel.ChannelType,
		ChannelValue: channel.ChannelValue,
		IsActive:    channel.IsActive,
		SrCreated:   channel.SrCreated.Unix(),
		SrUpdated:   channel.SrUpdated.Unix(),
	}
	if channel.ChannelName != nil {
		pbChannel.ChannelName = *channel.ChannelName
	}
	pbSub.Channel = pbChannel

	return connect.NewResponse(&sickrockpb.CreateUserNotificationSubscriptionResponse{
		Success:      true,
		Message:      "Notification subscription created successfully",
		Subscription: pbSub,
	}), nil
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
