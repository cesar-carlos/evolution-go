package send_service

import (
	"context"
	"errors"
	"fmt"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"strings"
	"time"
)

// SendLink bounds preview preparation and the single transmission together.
// A preview error degrades to text; caller cancellation never triggers a send.
func (s *sendService) SendLink(ctx context.Context, data *LinkStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if data == nil || instance == nil {
		return nil, errors.New("message and instance are required")
	}
	client, err := s.ensureSendClient(ctx, instance.Id)
	if err != nil {
		return nil, err
	}
	msg, previewErr := prepareLinkMessage(ctx, s.linkPreviewHTTPClient, data, func(ctx context.Context, raw []byte) (whatsmeow.UploadResponse, error) {
		return client.Upload(ctx, raw, whatsmeow.MediaLinkThumbnail)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if previewErr != nil {
		s.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Link preview incomplete; preserving text and available metadata", instance.Id)
	}
	return s.sendMessageContext(ctx, instance, msg, "ExtendedTextMessage", &SendDataStruct{Id: data.Id, Number: data.Number, Quoted: data.Quoted, Delay: data.Delay, MentionAll: data.MentionAll, MentionedJID: data.MentionedJID, FormatJid: data.FormatJid})
}

func (s *sendService) ensureSendClient(ctx context.Context, id string) (*whatsmeow.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	client := s.whatsmeowService.GetClient(id)
	if client == nil {
		if err := s.whatsmeowService.StartInstanceContext(ctx, id); err != nil {
			return nil, fmt.Errorf("start send session: %w", err)
		}
		waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for client == nil || !client.IsConnected() {
			select {
			case <-waitCtx.Done():
				return nil, waitCtx.Err()
			case <-ticker.C:
				client = s.whatsmeowService.GetClient(id)
			}
		}
	}
	if client.Store == nil || !client.IsConnected() || !client.IsLoggedIn() {
		return nil, errors.New("no active WhatsApp session")
	}
	return client, nil
}

func waitSendDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func (s *sendService) validateSendRecipient(ctx context.Context, client *whatsmeow.Client, data *SendDataStruct, instance *instance_model.Instance) (types.JID, error) {
	phone := data.Number
	validate := func() (types.JID, error) {
		return validateMessageFields(phone, data.FormatJid, &data.Quoted.MessageID, &data.Quoted.Participant)
	}
	if !s.config.CheckUserExists || strings.Contains(phone, "@g.us") || strings.Contains(phone, "@broadcast") || strings.Contains(phone, "@newsletter") || strings.Contains(phone, "@lid") {
		return validate()
	}
	for _, format := range []bool{false, true} {
		numbers, err := utils.PrepareNumbersForWhatsAppCheck([]string{phone}, &format)
		if err != nil {
			return types.JID{}, err
		}
		result, err := client.IsOnWhatsApp(ctx, numbers)
		if ctx.Err() != nil {
			return types.JID{}, ctx.Err()
		}
		if err != nil {
			return validate()
		}
		if len(result) > 0 && result[0].IsIn {
			raw := result[0].JID.String()
			noFormat := false
			return validateMessageFields(raw, &noFormat, &data.Quoted.MessageID, &data.Quoted.Participant)
		}
	}
	return types.JID{}, errors.New("number is not registered on WhatsApp")
}
