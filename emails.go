package leadpush

import "context"

// EmailRecipientType identifies a per-recipient message category.
type EmailRecipientType string

const (
	EmailRecipientTypeTo  EmailRecipientType = "to"
	EmailRecipientTypeBCC EmailRecipientType = "bcc"
)

// EmailSendMessage describes a per-recipient message accepted for delivery.
type EmailSendMessage struct {
	UUID      string             `json:"uuid"`
	Recipient string             `json:"recipient"`
	Type      EmailRecipientType `json:"type"`
	From      string             `json:"from"`
	Status    string             `json:"status"`
}

// EmailSend describes an accepted email send.
type EmailSend struct {
	Accepted     bool               `json:"accepted"`
	MessageCount int                `json:"message_count"`
	Messages     []EmailSendMessage `json:"messages"`
}

// SendEmailParams is accepted by EmailsService.Send.
type SendEmailParams struct {
	From    string            `json:"from"`
	Subject string            `json:"subject"`
	HTML    *string           `json:"html,omitzero"`
	Text    *string           `json:"text,omitzero"`
	To      []string          `json:"to,omitzero"`
	BCC     []string          `json:"bcc,omitzero"`
	ReplyTo *string           `json:"reply_to,omitzero"`
	Headers map[string]string `json:"headers,omitzero"`
}

// EmailsService provides email sending API operations.
type EmailsService struct {
	client *Client
}

// Send queues an email for delivery.
func (service *EmailsService) Send(ctx context.Context, params SendEmailParams) (*EmailSend, error) {
	var response resourceResponse[EmailSend]
	if err := service.client.Post(ctx, []string{"emails"}, params, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}
