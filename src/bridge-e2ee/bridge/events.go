package bridge

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waArmadilloApplication"
	"go.mau.fi/whatsmeow/proto/waArmadilloXMA"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waConsumerApplication"
	"go.mau.fi/whatsmeow/types/events"

	"go.mau.fi/mautrix-meta/pkg/messagix"
	"go.mau.fi/mautrix-meta/pkg/messagix/table"
)

// The fake stickers that are sent when someone presses the thumbs-up
// button in Messenger. They are handled specially by the Messenger
// web client instead of being displayed as normal stickers. There are
// three variants depending on how long the sending user held down the
// send button.
const (
	facebookThumbsUpSmallStickerID  int64 = 369239263222822
	facebookThumbsUpMediumStickerID int64 = 369239343222814
	facebookThumbsUpLargeStickerID  int64 = 369239383222810
)

// EventType represents the type of event
type EventType string

const (
	EventTypeReady         EventType = "ready"
	EventTypeReconnected   EventType = "reconnected"
	EventTypeDisconnected  EventType = "disconnected"
	EventTypeError         EventType = "error"
	EventTypeRaw           EventType = "raw"
	EventTypeMessage       EventType = "message"
	EventTypeMessageEdit   EventType = "messageEdit"
	EventTypeMessageUnsend EventType = "messageUnsend"
	EventTypeReaction      EventType = "reaction"
	EventTypeTyping        EventType = "typing"
	EventTypePresence      EventType = "presence"
	EventTypeReadReceipt   EventType = "readReceipt"
	EventTypeE2EEConnected EventType = "e2eeConnected"
	EventTypeE2EEMessage   EventType = "e2eeMessage"
	EventTypeE2EEReaction  EventType = "e2eeReaction"
	EventTypeE2EEReceipt   EventType = "e2eeReceipt"
	EventDeviceDataChanged EventType = "deviceDataChanged"
)

// Event represents a generic event
type Event struct {
	Type      EventType   `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// UserInfo holds user information
type UserInfo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// InitialData holds initial sync data
type InitialData struct {
	Threads  []*Thread  `json:"threads"`
	Messages []*Message `json:"messages"`
}

// Thread represents a conversation thread
type Thread struct {
	ID                      int64  `json:"id"`
	Type                    int    `json:"type"`
	Name                    string `json:"name"`
	LastActivityTimestampMs int64  `json:"lastActivityTimestampMs"`
	Snippet                 string `json:"snippet"`
}

// Attachment represents a media attachment
type Attachment struct {
	Type        string  `json:"type"` // "image", "video", "audio", "file", "sticker", "gif", "voice", "location", "link"
	URL         string  `json:"url,omitempty"`
	FileName    string  `json:"fileName,omitempty"`
	MimeType    string  `json:"mimeType,omitempty"`
	FileSize    int64   `json:"fileSize,omitempty"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	Duration    int     `json:"duration,omitempty"` // in seconds for audio/video
	StickerID   int64   `json:"stickerId,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	PreviewURL  string  `json:"previewUrl,omitempty"`
	Description string  `json:"description,omitempty"` // For link attachments
	SourceText  string  `json:"sourceText,omitempty"`  // Domain/source for link attachments
	// For E2EE media download
	MediaKey       []byte `json:"mediaKey,omitempty"`
	MediaSHA256    []byte `json:"mediaSha256,omitempty"`
	MediaEncSHA256 []byte `json:"mediaEncSha256,omitempty"`
	DirectPath     string `json:"directPath,omitempty"`
}

// ReplyTo represents reply info
type ReplyTo struct {
	MessageID string `json:"messageId"`
	SenderID  int64  `json:"senderId,omitempty"`
	Text      string `json:"text,omitempty"`
}

// Mention represents a mention
type Mention struct {
	UserID int64  `json:"userId"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	Type   string `json:"type,omitempty"` // "user", "page", "group"
}

// Message represents a regular (non-E2EE) message
type Message struct {
	ID          string        `json:"id"`
	ThreadID    int64         `json:"threadId"`
	ThreadName  string        `json:"threadName,omitempty"`
	ThreadType  int           `json:"threadType,omitempty"`
	SenderID    int64         `json:"senderId"`
	Text        string        `json:"text"`
	TimestampMs int64         `json:"timestampMs"`
	Attachments []*Attachment `json:"attachments,omitempty"`
	ReplyTo     *ReplyTo      `json:"replyTo,omitempty"`
	Mentions    []*Mention    `json:"mentions,omitempty"`
	IsAdminMsg  bool          `json:"isAdminMsg,omitempty"`
}

// MessageEditEvent represents a message edit
type MessageEditEvent struct {
	MessageID   string `json:"messageId"`
	ThreadID    int64  `json:"threadId"`
	NewText     string `json:"newText"`
	EditCount   int64  `json:"editCount"`
	TimestampMs int64  `json:"timestampMs"`
}

// ReadReceiptEvent represents a read receipt
type ReadReceiptEvent struct {
	ThreadID                 int64 `json:"threadId"`
	ReaderID                 int64 `json:"readerId"`
	ReadWatermarkTimestampMs int64 `json:"readWatermarkTimestampMs"`
	TimestampMs              int64 `json:"timestampMs,omitempty"`
}

// ReactionEvent represents a reaction event
type ReactionEvent struct {
	MessageID   string `json:"messageId"`
	ThreadID    int64  `json:"threadId"`
	ActorID     int64  `json:"actorId"`
	Reaction    string `json:"reaction"`
	TimestampMs int64  `json:"timestampMs"`
}

// TypingEvent represents a typing event
type TypingEvent struct {
	ThreadID int64 `json:"threadId"`
	SenderID int64 `json:"senderId"`
	IsTyping bool  `json:"isTyping"`
}

// ErrorEvent represents an error event
type ErrorEvent struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

// RawEventSource represents the source of a raw event
type RawEventSource string

const (
	RawEventSourceLightSpeed RawEventSource = "lightspeed" // LightSpeed events (non-E2EE)
	RawEventSourceWhatsmeow  RawEventSource = "whatsmeow"  // WhatsApp/WhatsMe events (E2EE)
)

// RawEvent represents a raw event from internal sources
// This is useful for debugging or handling events not explicitly supported
type RawEvent struct {
	// From indicates the source of the event
	From RawEventSource `json:"from"`
	// Type is the Go type name of the original event
	Type string `json:"type"`
	// Data contains the raw event data (JSON serialized)
	Data interface{} `json:"data"`
}

// E2EEMessage represents an end-to-end encrypted message
type E2EEMessage struct {
	ID          string        `json:"id"`
	ThreadID    int64         `json:"threadId"`
	ChatJID     string        `json:"chatJid"`
	SenderJID   string        `json:"senderJid"`
	SenderID    int64         `json:"senderId"`
	Text        string        `json:"text"`
	TimestampMs int64         `json:"timestampMs"`
	Attachments []*Attachment `json:"attachments,omitempty"`
	ReplyTo     *ReplyTo      `json:"replyTo,omitempty"`
	Mentions    []*Mention    `json:"mentions,omitempty"`
}

// getEventTypeName returns the type name of an event
func getEventTypeName(evt any) string {
	if evt == nil {
		return "nil"
	}
	t := reflect.TypeOf(evt)
	if t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	}
	return t.Name()
}

// handleEvent handles messagix events
func (c *Client) handleEvent(ctx context.Context, evt any) {
	// Emit raw event for all incoming LightSpeed events
	c.emitEvent(EventTypeRaw, &RawEvent{
		From: RawEventSourceLightSpeed,
		Type: getEventTypeName(evt),
		Data: evt,
	})

	switch e := evt.(type) {
	case *messagix.Event_Ready:
		c.emitEvent(EventTypeReady, map[string]any{
			"isNewSession": e.IsNewSession,
		})

	case *messagix.Event_Reconnected:
		c.emitEvent(EventTypeReconnected, nil)

	case *messagix.Event_SocketError:
		c.emitEvent(EventTypeError, &ErrorEvent{
			Message: e.Err.Error(),
		})

	case *messagix.Event_PermanentError:
		c.emitEvent(EventTypeError, &ErrorEvent{
			Message: e.Err.Error(),
			Code:    1,
		})

	case *messagix.Event_PublishResponse:
		if e.Table != nil {
			c.handleTable(e.Table)
		}
	}
}

// handleTable processes a table from publish response
func (c *Client) handleTable(tbl *table.LSTable) {
	for _, thread := range tbl.LSDeleteThenInsertThread {
		c.cacheThread(convertThread(thread))
	}

	// Process wrapped messages (includes attachments info)
	_, insert := tbl.WrapMessages()

	// Track handled message IDs to avoid duplicates
	handledMsgIds := make(map[string]bool)

	// Handle inserted messages (new real-time messages)
	for _, msg := range insert {
		if msg.MessageId != "" {
			if handledMsgIds[msg.MessageId] {
				continue
			}
			handledMsgIds[msg.MessageId] = true
		}
		c.emitEvent(EventTypeMessage, c.convertWrappedMessage(msg))
	}

	// Handle simple inserted messages (fallback) - skip if already handled
	for _, msg := range tbl.LSInsertMessage {
		if handledMsgIds[msg.MessageId] {
			continue
		}
		threadName, threadType := c.threadMeta(msg.ThreadKey)
		c.emitEvent(EventTypeMessage, &Message{
			ID:          msg.MessageId,
			ThreadID:    msg.ThreadKey,
			ThreadName:  threadName,
			ThreadType:  threadType,
			SenderID:    msg.SenderId,
			Text:        msg.Text,
			TimestampMs: msg.TimestampMs,
		})
	}

	// Handle message edits
	for _, edit := range tbl.LSEditMessage {
		c.emitEvent(EventTypeMessageEdit, &MessageEditEvent{
			MessageID:   edit.MessageID,
			ThreadID:    0,
			NewText:     edit.Text,
			EditCount:   edit.EditCount,
			TimestampMs: timeNowMs(),
		})
	}

	// Handle message deletes
	for _, del := range tbl.LSDeleteMessage {
		c.emitEvent(EventTypeMessageUnsend, map[string]any{
			"messageId": del.MessageId,
			"threadId":  del.ThreadKey,
		})
	}

	// Handle DeleteThenInsert for unsend
	for _, del := range tbl.LSDeleteThenInsertMessage {
		if del.IsUnsent {
			c.emitEvent(EventTypeMessageUnsend, map[string]any{
				"messageId": del.MessageId,
				"threadId":  del.ThreadKey,
			})
		}
	}

	// Handle read receipts
	for _, receipt := range tbl.LSUpdateReadReceipt {
		c.emitEvent(EventTypeReadReceipt, &ReadReceiptEvent{
			ThreadID:                 receipt.ThreadKey,
			ReaderID:                 receipt.ContactId,
			ReadWatermarkTimestampMs: receipt.ReadWatermarkTimestampMs,
			TimestampMs:              receipt.ReadActionTimestampMs,
		})
	}

	// Handle self read (mark thread read)
	for _, read := range tbl.LSMarkThreadReadV2 {
		c.emitEvent(EventTypeReadReceipt, &ReadReceiptEvent{
			ThreadID:                 read.ThreadKey,
			ReaderID:                 c.FBID, // Self
			ReadWatermarkTimestampMs: read.LastReadWatermarkTimestampMs,
			TimestampMs:              timeNowMs(),
		})
	}

	// Handle reactions
	for _, r := range tbl.LSUpsertReaction {
		c.emitEvent(EventTypeReaction, &ReactionEvent{
			MessageID:   r.MessageId,
			ThreadID:    r.ThreadKey,
			ActorID:     r.ActorId,
			Reaction:    r.Reaction,
			TimestampMs: r.TimestampMs,
		})
	}

	// Handle unreactions (reaction removed) with deduplication
	for _, r := range tbl.LSDeleteReaction {
		unreactionKey := fmt.Sprintf("%s:%d", r.MessageId, r.ActorId)
		now := time.Now().UnixMilli()

		c.recentUnreactionsMu.RLock()
		lastTime, exists := c.recentUnreactions[unreactionKey]
		c.recentUnreactionsMu.RUnlock()

		if exists && (now-lastTime) < 500 {
			continue
		}

		c.recentUnreactionsMu.Lock()
		c.recentUnreactions[unreactionKey] = now
		for k, t := range c.recentUnreactions {
			if now-t > 5000 {
				delete(c.recentUnreactions, k)
			}
		}
		c.recentUnreactionsMu.Unlock()

		c.emitEvent(EventTypeReaction, &ReactionEvent{
			MessageID:   r.MessageId,
			ThreadID:    r.ThreadKey,
			ActorID:     r.ActorId,
			Reaction:    "", // Empty means reaction removed
			TimestampMs: 0,
		})
	}

	// Handle typing indicators
	for _, typing := range tbl.LSUpdateTypingIndicator {
		c.emitEvent(EventTypeTyping, &TypingEvent{
			ThreadID: typing.ThreadKey,
			SenderID: typing.SenderId,
			IsTyping: typing.IsTyping,
		})
	}
}

// parseMentions parses comma-separated mention strings into Mention structs
func parseMentions(offsets, lengths, ids string) []*Mention {
	return parseMentionsWithTypes(offsets, lengths, ids, "")
}

// parseMentionsWithTypes parses comma-separated mention strings with types
func parseMentionsWithTypes(offsets, lengths, ids, types string) []*Mention {
	if offsets == "" || ids == "" {
		return nil
	}

	offsetParts := strings.Split(offsets, ",")
	lengthParts := strings.Split(lengths, ",")
	idParts := strings.Split(ids, ",")
	typeParts := strings.Split(types, ",")

	count := len(offsetParts)
	if len(idParts) < count {
		count = len(idParts)
	}

	mentions := make([]*Mention, 0, count)
	for i := 0; i < count; i++ {
		offset, err := strconv.Atoi(strings.TrimSpace(offsetParts[i]))
		if err != nil {
			continue
		}
		length := 0
		if i < len(lengthParts) {
			length, _ = strconv.Atoi(strings.TrimSpace(lengthParts[i]))
		}
		userID, err := strconv.ParseInt(strings.TrimSpace(idParts[i]), 10, 64)
		if err != nil {
			continue
		}
		mentionType := "user"
		if i < len(typeParts) {
			switch strings.TrimSpace(typeParts[i]) {
			case "p":
				mentionType = "user"
			case "t":
				mentionType = "thread"
			case "g":
				mentionType = "group"
			}
		}
		mentions = append(mentions, &Mention{
			UserID: userID,
			Offset: offset,
			Length: length,
			Type:   mentionType,
		})
	}
	return mentions
}

// convertWrappedMessage converts a wrapped message with attachments
func (c *Client) convertWrappedMessage(msg *table.WrappedMessage) *Message {
	if len(msg.Stickers) == 1 {
		stickerID := msg.Stickers[0].TargetId
		if stickerID == facebookThumbsUpLargeStickerID ||
			stickerID == facebookThumbsUpMediumStickerID ||
			stickerID == facebookThumbsUpSmallStickerID {
			msg.Text = "👍"
			msg.Stickers = nil
		}
	}

	threadName, threadType := c.threadMeta(msg.ThreadKey)
	m := &Message{
		ID:          msg.MessageId,
		ThreadID:    msg.ThreadKey,
		ThreadName:  threadName,
		ThreadType:  threadType,
		SenderID:    msg.SenderId,
		Text:        msg.Text,
		TimestampMs: msg.TimestampMs,
		IsAdminMsg:  msg.IsAdminMessage,
		Attachments: []*Attachment{},
		Mentions:    []*Mention{},
	}

	// Handle reply
	if msg.ReplySourceId != "" {
		m.ReplyTo = &ReplyTo{
			MessageID: msg.ReplySourceId,
			SenderID:  msg.ReplyToUserId,
			Text:      msg.ReplySnippet,
		}
	}

	// Parse mentions
	if mentions := parseMentionsWithTypes(msg.MentionOffsets, msg.MentionLengths, msg.MentionIds, msg.MentionTypes); mentions != nil {
		m.Mentions = mentions
	}

	// Handle blob attachments
	seenBlobFBIDs := make(map[string]bool)
	for _, blob := range msg.BlobAttachments {
		if blob.AttachmentFbid != "" {
			if seenBlobFBIDs[blob.AttachmentFbid] {
				continue
			}
			seenBlobFBIDs[blob.AttachmentFbid] = true
		}
		att := c.convertBlobAttachment(blob)
		if att != nil {
			m.Attachments = append(m.Attachments, att)
		}
	}

	// Handle stickers
	for _, sticker := range msg.Stickers {
		var stickerID int64
		if sticker.AttachmentFbid != "" {
			stickerID, _ = strconv.ParseInt(sticker.AttachmentFbid, 10, 64)
		}
		if stickerID == 0 {
			stickerID = sticker.TargetId
		}
		m.Attachments = append(m.Attachments, &Attachment{
			Type:      "sticker",
			URL:       sticker.PreviewUrl,
			StickerID: stickerID,
			Width:     int(sticker.PreviewWidth),
			Height:    int(sticker.PreviewHeight),
		})
	}

	// Handle XMA attachments
	for _, xma := range msg.XMAAttachments {
		if xma.CTA != nil && xma.CTA.Type_ == "xma_map" {
			if xma.CTA.NativeUrl != "" {
				parts := strings.Split(xma.CTA.NativeUrl, ",")
				if len(parts) == 2 {
					lat, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
					lng, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
					m.Attachments = append(m.Attachments, &Attachment{
						Type:        "location",
						Latitude:    lat,
						Longitude:   lng,
						FileName:    xma.TitleText,
						Description: xma.SubtitleText,
					})
					continue
				}
			}
			m.Attachments = append(m.Attachments, &Attachment{
				Type:        "location",
				FileName:    xma.TitleText,
				Description: xma.SubtitleText,
			})
			continue
		}

		if xma.CTA != nil && strings.HasPrefix(xma.CTA.Type_, "xma_poll_") {
			continue
		}

		var linkURL string
		if xma.CTA != nil && xma.CTA.ActionUrl != "" {
			linkURL = extractURLFromLPHP(xma.CTA.ActionUrl)
		} else if xma.ActionUrl != "" {
			linkURL = extractURLFromLPHP(xma.ActionUrl)
		}

		if linkURL != "" || xma.PreviewUrl != "" {
			m.Attachments = append(m.Attachments, &Attachment{
				Type:        "link",
				URL:         linkURL,
				PreviewURL:  xma.PreviewUrl,
				FileName:    xma.TitleText,
				Description: xma.SubtitleText,
				SourceText:  xma.SourceText,
				Width:       int(xma.PreviewWidth),
				Height:      int(xma.PreviewHeight),
			})
		}
	}

	return m
}

func (c *Client) cacheThread(thread *Thread) {
	if thread == nil || thread.ID == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.threadCache == nil {
		c.threadCache = make(map[int64]*Thread)
	}
	c.threadCache[thread.ID] = thread
}

func (c *Client) threadMeta(threadID int64) (string, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	thread := c.threadCache[threadID]
	if thread == nil {
		return "", 0
	}
	return thread.Name, thread.Type
}

// convertBlobAttachment converts a blob attachment to our format
func (c *Client) convertBlobAttachment(blob *table.LSInsertBlobAttachment) *Attachment {
	att := &Attachment{
		FileName: blob.Filename,
		MimeType: blob.AttachmentMimeType,
		FileSize: blob.Filesize,
	}

	switch blob.AttachmentType {
	case table.AttachmentTypeImage, table.AttachmentTypeEphemeralImage:
		att.Type = "image"
		att.URL = blob.PreviewUrl
		att.Width = int(blob.PreviewWidth)
		att.Height = int(blob.PreviewHeight)
	case table.AttachmentTypeAnimatedImage:
		att.Type = "gif"
		att.URL = blob.PlayableUrl
		if att.URL == "" {
			att.URL = blob.PreviewUrl
		}
		att.PreviewURL = blob.PreviewUrl
		att.Width = int(blob.PreviewWidth)
		att.Height = int(blob.PreviewHeight)
	case table.AttachmentTypeVideo, table.AttachmentTypeEphemeralVideo:
		att.Type = "video"
		att.URL = blob.PlayableUrl
		att.PreviewURL = blob.PreviewUrl
		att.Width = int(blob.PreviewWidth)
		att.Height = int(blob.PreviewHeight)
		att.Duration = int(blob.PlayableDurationMs / 1000)
	case table.AttachmentTypeAudio:
		att.Type = "audio"
		att.URL = blob.PlayableUrl
		att.Duration = int(blob.PlayableDurationMs / 1000)
	case table.AttachmentTypeFile:
		att.Type = "file"
		if blob.PlayableUrl != "" {
			att.URL = blob.PlayableUrl
		} else {
			att.URL = blob.PreviewUrl
		}
	case table.AttachmentTypeSoundBite:
		att.Type = "voice"
		att.URL = blob.PlayableUrl
		att.Duration = int(blob.PlayableDurationMs / 1000)
	default:
		att.Type = "file"
		if blob.PlayableUrl != "" {
			att.URL = blob.PlayableUrl
		} else if blob.PreviewUrl != "" {
			att.URL = blob.PreviewUrl
		}
	}

	return att
}

// handleE2EEEvent handles WhatsApp E2EE events
func (c *Client) handleE2EEEvent(evt interface{}) {
	c.emitEvent(EventTypeRaw, &RawEvent{
		From: RawEventSourceWhatsmeow,
		Type: getEventTypeName(evt),
		Data: evt,
	})

	switch e := evt.(type) {
	case *events.Connected:
		c.emitEvent(EventTypeE2EEConnected, nil)

	case *events.Disconnected:
		c.emitEvent(EventTypeDisconnected, map[string]any{
			"isE2EE": true,
		})

	case *events.FBMessage:
		var senderID int64
		if e.Info.Sender.User != "" {
			senderID, _ = strconv.ParseInt(e.Info.Sender.User, 10, 64)
		}

		if isE2EEReactionMessage(e) {
			reaction := extractE2EEReaction(e)
			c.emitEvent(EventTypeE2EEReaction, map[string]any{
				"messageId": extractE2EEReactionMessageID(e),
				"chatJid":   e.Info.Chat.String(),
				"senderJid": e.Info.Sender.String(),
				"senderId":  senderID,
				"reaction":  reaction,
			})
			return
		}

		if isE2EEEditMessage(e) {
			editInfo := extractE2EEEditInfo(e)
			if editInfo != nil {
				c.emitEvent(EventTypeMessageEdit, &MessageEditEvent{
					MessageID:   editInfo.MessageID,
					ThreadID:    0,
					NewText:     editInfo.NewText,
					EditCount:   1,
					TimestampMs: e.Info.Timestamp.UnixMilli(),
				})
			}
			return
		}

		if isE2EERevokeMessage(e) {
			revokedMsgID := extractE2EERevokedMessageID(e)
			if revokedMsgID != "" {
				c.emitEvent(EventTypeMessageUnsend, map[string]any{
					"messageId": revokedMsgID,
					"threadId":  e.Info.Chat.String(),
					"isE2EE":    true,
				})
			}
			return
		}

		msg := c.extractE2EEMessage(e, senderID)
		if msg == nil {
			return
		}
		c.emitEvent(EventTypeE2EEMessage, msg)

	case *events.Receipt:
		c.emitEvent(EventTypeE2EEReceipt, map[string]any{
			"type":       string(e.Type),
			"chat":       e.Chat.String(),
			"sender":     e.Sender.String(),
			"messageIds": e.MessageIDs,
		})
	}
}

// emitEvent emits an event to the channel
func (c *Client) emitEvent(eventType EventType, data interface{}) {
	select {
	case c.eventChan <- &Event{
		Type:      eventType,
		Data:      data,
		Timestamp: timeNowMs(),
	}:
	default:
		c.Logger.Warn().Str("type", string(eventType)).Msg("Event channel full, dropping event")
	}
}

// extractE2EEText extracts text from an E2EE message
func extractE2EEText(e *events.FBMessage) string {
	if e.Message == nil {
		return ""
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if c := p.GetContent(); c != nil {
				if mt, ok := c.GetContent().(*waConsumerApplication.ConsumerApplication_Content_MessageText); ok {
					return mt.MessageText.GetText()
				}
				if et, ok := c.GetContent().(*waConsumerApplication.ConsumerApplication_Content_ExtendedTextMessage); ok {
					extMsg := et.ExtendedTextMessage
					if extMsg == nil {
						return ""
					}
					if textMsg := extMsg.GetText(); textMsg != nil {
						if text := textMsg.GetText(); text != "" {
							return text
						}
					}
					if matched := extMsg.GetMatchedText(); matched != "" {
						return matched
					}
					if canonical := extMsg.GetCanonicalURL(); canonical != "" {
						return canonical
					}
				}
			}
		}
	}

	if armadillo, ok := e.Message.(*waArmadilloApplication.Armadillo); ok {
		if payload := armadillo.GetPayload(); payload != nil {
			if content := payload.GetContent(); content != nil {
				if extMsg := content.GetExtendedContentMessage(); extMsg != nil {
					if text := extMsg.GetMessageText(); text != "" {
						return text
					}
					if title := extMsg.GetTitleText(); title != "" {
						return title
					}
					if ctas := extMsg.GetCtas(); len(ctas) > 0 {
						for _, cta := range ctas {
							if actionURL := cta.GetActionURL(); actionURL != "" {
								if parsedURL := extractURLFromLPHP(actionURL); parsedURL != "" {
									return parsedURL
								}
								return actionURL
							}
							if nativeURL := cta.GetNativeURL(); nativeURL != "" {
								return nativeURL
							}
						}
					}
				}
				if searMsg := content.GetExtendedMessageContentWithSear(); searMsg != nil {
					if nativeURL := searMsg.GetNativeURL(); nativeURL != "" {
						return nativeURL
					}
				}
			}
		}
	}

	return ""
}

// isE2EEReactionMessage checks if the message is a reaction
func isE2EEReactionMessage(e *events.FBMessage) bool {
	if e.Message == nil {
		return false
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if c := p.GetContent(); c != nil {
				_, isReaction := c.GetContent().(*waConsumerApplication.ConsumerApplication_Content_ReactionMessage)
				return isReaction
			}
		}
	}

	return false
}

// extractE2EEReaction extracts reaction emoji
func extractE2EEReaction(e *events.FBMessage) string {
	if e.Message == nil {
		return ""
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if c := p.GetContent(); c != nil {
				if rm, ok := c.GetContent().(*waConsumerApplication.ConsumerApplication_Content_ReactionMessage); ok {
					return rm.ReactionMessage.GetText()
				}
			}
		}
	}

	return ""
}

// extractE2EEReactionMessageID extracts message ID reacted to
func extractE2EEReactionMessageID(e *events.FBMessage) string {
	if e.Message == nil {
		return ""
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if c := p.GetContent(); c != nil {
				if rm, ok := c.GetContent().(*waConsumerApplication.ConsumerApplication_Content_ReactionMessage); ok {
					if key := rm.ReactionMessage.GetKey(); key != nil {
						return key.GetID()
					}
				}
			}
		}
	}

	return ""
}

func timeNowMs() int64 {
	return time.Now().UnixMilli()
}

// ErrE2EENotConnected error when E2EE is not connected
var ErrE2EENotConnected = fmt.Errorf("E2EE not connected")

// E2EEEditInfo holds edit information
type E2EEEditInfo struct {
	MessageID string
	NewText   string
}

// isE2EEEditMessage checks if the message is an edit
func isE2EEEditMessage(e *events.FBMessage) bool {
	if e.Message == nil {
		return false
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if c := p.GetContent(); c != nil {
				_, isEdit := c.GetContent().(*waConsumerApplication.ConsumerApplication_Content_EditMessage)
				return isEdit
			}
		}
	}

	return false
}

// extractE2EEEditInfo extracts edit info
func extractE2EEEditInfo(e *events.FBMessage) *E2EEEditInfo {
	if e.Message == nil {
		return nil
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if c := p.GetContent(); c != nil {
				if em, ok := c.GetContent().(*waConsumerApplication.ConsumerApplication_Content_EditMessage); ok {
					edit := em.EditMessage
					return &E2EEEditInfo{
						MessageID: edit.GetKey().GetID(),
						NewText:   edit.GetMessage().GetText(),
					}
				}
			}
		}
	}

	return nil
}

// isE2EERevokeMessage checks if the message is unsend
func isE2EERevokeMessage(e *events.FBMessage) bool {
	if e.Message == nil {
		return false
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if appData := p.GetApplicationData(); appData != nil {
				return appData.GetRevoke() != nil
			}
		}
	}

	return false
}

// extractE2EERevokedMessageID extracts revoked message ID
func extractE2EERevokedMessageID(e *events.FBMessage) string {
	if e.Message == nil {
		return ""
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if appData := p.GetApplicationData(); appData != nil {
				if revoke := appData.GetRevoke(); revoke != nil {
					if key := revoke.GetKey(); key != nil {
						return key.GetID()
					}
				}
			}
		}
	}

	return ""
}

// extractE2EEMentions extracts mentions
func extractE2EEMentions(text *waCommon.MessageText) []*Mention {
	if text == nil {
		return nil
	}
	jids := text.GetMentionedJID()
	if len(jids) == 0 {
		return nil
	}

	mentions := make([]*Mention, 0, len(jids))
	textContent := text.GetText()

	for _, jid := range jids {
		var userID int64
		atIdx := strings.Index(jid, "@")
		if atIdx > 0 {
			userID, _ = strconv.ParseInt(jid[:atIdx], 10, 64)
		}
		if userID == 0 {
			continue
		}

		mentionText := "@" + jid
		offset := strings.Index(textContent, mentionText)
		length := len(mentionText)

		mentions = append(mentions, &Mention{
			UserID: userID,
			Offset: offset,
			Length: length,
			Type:   "user",
		})
	}
	return mentions
}

// extractE2EEReplyTo extracts reply info
func extractE2EEReplyTo(e *events.FBMessage) *ReplyTo {
	if e.FBApplication == nil {
		return nil
	}
	metadata := e.FBApplication.GetMetadata()
	if metadata == nil {
		return nil
	}
	qm := metadata.GetQuotedMessage()
	if qm == nil {
		return nil
	}

	replyTo := &ReplyTo{
		MessageID: qm.GetStanzaID(),
	}

	participant := qm.GetParticipant()
	if participant != "" {
		atIdx := strings.Index(participant, "@")
		if atIdx > 0 {
			replyTo.SenderID, _ = strconv.ParseInt(participant[:atIdx], 10, 64)
		}
	}

	return replyTo
}

// extractE2EEMessage extracts full message content
func (c *Client) extractE2EEMessage(e *events.FBMessage, senderID int64) *E2EEMessage {
	var threadID int64
	if e.Info.Chat.User != "" {
		threadID, _ = strconv.ParseInt(e.Info.Chat.User, 10, 64)
	}

	msg := &E2EEMessage{
		ID:          e.Info.ID,
		ThreadID:    threadID,
		ChatJID:     e.Info.Chat.String(),
		SenderJID:   e.Info.Sender.String(),
		SenderID:    senderID,
		Text:        extractE2EEText(e),
		TimestampMs: e.Info.Timestamp.UnixMilli(),
		Attachments: []*Attachment{},
		Mentions:    []*Mention{},
	}

	if replyTo := extractE2EEReplyTo(e); replyTo != nil {
		msg.ReplyTo = replyTo
	}

	if e.Message == nil {
		return msg
	}

	if ca, ok := e.Message.(*waConsumerApplication.ConsumerApplication); ok {
		if p := ca.GetPayload(); p != nil {
			if content := p.GetContent(); content != nil {

				if img, ok := content.GetContent().(*waConsumerApplication.ConsumerApplication_Content_ImageMessage); ok {
					att := c.extractE2EEImageAttachment(img.ImageMessage)
					msg.Attachments = append(msg.Attachments, att)
					if caption := img.ImageMessage.GetCaption(); caption != nil {
						msg.Text = caption.GetText()
						if mentions := extractE2EEMentions(caption); mentions != nil {
							msg.Mentions = mentions
						}
					}
				}

				if vid, ok := content.GetContent().(*waConsumerApplication.ConsumerApplication_Content_VideoMessage); ok {
					att := c.extractE2EEVideoAttachment(vid.VideoMessage)
					msg.Attachments = append(msg.Attachments, att)
					if caption := vid.VideoMessage.GetCaption(); caption != nil {
						msg.Text = caption.GetText()
						if mentions := extractE2EEMentions(caption); mentions != nil {
							msg.Mentions = mentions
						}
					}
				}

				if audio, ok := content.GetContent().(*waConsumerApplication.ConsumerApplication_Content_AudioMessage); ok {
					att := c.extractE2EEAudioAttachment(audio.AudioMessage)
					msg.Attachments = append(msg.Attachments, att)
				}

				if doc, ok := content.GetContent().(*waConsumerApplication.ConsumerApplication_Content_DocumentMessage); ok {
					att := c.extractE2EEDocumentAttachment(doc.DocumentMessage)
					msg.Attachments = append(msg.Attachments, att)
				}

				if sticker, ok := content.GetContent().(*waConsumerApplication.ConsumerApplication_Content_StickerMessage); ok {
					att := c.extractE2EEStickerAttachment(sticker.StickerMessage)
					msg.Attachments = append(msg.Attachments, att)
				}

				if loc, ok := content.GetContent().(*waConsumerApplication.ConsumerApplication_Content_LocationMessage); ok {
					att := &Attachment{
						Type:      "location",
						Latitude:  loc.LocationMessage.GetLocation().GetDegreesLatitude(),
						Longitude: loc.LocationMessage.GetLocation().GetDegreesLongitude(),
						FileName:  loc.LocationMessage.GetAddress(),
					}
					msg.Attachments = append(msg.Attachments, att)
				}

				if text, ok := content.GetContent().(*waConsumerApplication.ConsumerApplication_Content_MessageText); ok {
					if mentions := extractE2EEMentions(text.MessageText); mentions != nil {
						msg.Mentions = mentions
					}
				}

				if ext, ok := content.GetContent().(*waConsumerApplication.ConsumerApplication_Content_ExtendedTextMessage); ok {
					if extMsg := ext.ExtendedTextMessage; extMsg != nil {
						var textContent, matchedText, canonicalURL string
						if textMsg := extMsg.GetText(); textMsg != nil {
							textContent = textMsg.GetText()
							if textContent != "" {
								msg.Text = textContent
							}
							if mentions := extractE2EEMentions(textMsg); mentions != nil {
								msg.Mentions = mentions
							}
						}
						matchedText = extMsg.GetMatchedText()
						canonicalURL = extMsg.GetCanonicalURL()

						if msg.Text == "" && matchedText != "" {
							msg.Text = matchedText
						}
						if msg.Text == "" && canonicalURL != "" {
							msg.Text = canonicalURL
						}
						linkURL := canonicalURL
						if linkURL == "" {
							linkURL = matchedText
						}
						if linkURL != "" {
							att := &Attachment{
								Type:        "link",
								URL:         linkURL,
								FileName:    extMsg.GetTitle(),
								Description: extMsg.GetDescription(),
							}
							if thumb, err := extMsg.DecodeThumbnail(); err == nil && thumb != nil {
								if ancillary := thumb.GetAncillary(); ancillary != nil {
									att.Width = int(ancillary.GetWidth())
									att.Height = int(ancillary.GetHeight())
								}
							}
							msg.Attachments = append(msg.Attachments, att)
						}
					}
				}
			}
		}
	}

	if armadillo, ok := e.Message.(*waArmadilloApplication.Armadillo); ok {
		payload := armadillo.GetPayload()
		if payload == nil {
			return msg
		}

		if content := payload.GetContent(); content != nil {
			if extMsg := content.GetExtendedContentMessage(); extMsg != nil {
				targetType := extMsg.GetTargetType()
				if targetType == waArmadilloXMA.ExtendedContentMessage_MSG_LOCATION_SHARING_V2 {
					return nil
				}

				att := c.extractArmadilloLinkAttachment(extMsg)
				if att != nil {
					msg.Attachments = append(msg.Attachments, att)
				}
			}

			if searMsg := content.GetExtendedMessageContentWithSear(); searMsg != nil {
				if nativeURL := searMsg.GetNativeURL(); nativeURL != "" {
					att := &Attachment{
						Type: "link",
						URL:  nativeURL,
					}
					msg.Attachments = append(msg.Attachments, att)
				}
			}
		}
	}

	return msg
}

// extractArmadilloLinkAttachment extracts link attachment from Armadillo
func (c *Client) extractArmadilloLinkAttachment(extMsg *waArmadilloXMA.ExtendedContentMessage) *Attachment {
	if extMsg == nil {
		return nil
	}

	var linkURL string
	if ctas := extMsg.GetCtas(); len(ctas) > 0 {
		for _, cta := range ctas {
			if actionURL := cta.GetActionURL(); actionURL != "" {
				if parsedURL := extractURLFromLPHP(actionURL); parsedURL != "" {
					linkURL = parsedURL
				} else {
					linkURL = actionURL
				}
				break
			}
			if nativeURL := cta.GetNativeURL(); nativeURL != "" {
				linkURL = nativeURL
				break
			}
		}
	}

	if linkURL == "" {
		return nil
	}

	att := &Attachment{
		Type:        "link",
		URL:         linkURL,
		FileName:    extMsg.GetTitleText(),
		Description: extMsg.GetSubtitleText(),
	}

	if header := extMsg.GetHeaderTitle(); header != "" && att.FileName == "" {
		att.FileName = header
	}

	if overlay := extMsg.GetOverlayTitle(); overlay != "" {
		att.SourceText = overlay
	}

	return att
}

// extractE2EEImageAttachment extracts image attachment metadata
func (c *Client) extractE2EEImageAttachment(img *waConsumerApplication.ConsumerApplication_ImageMessage) *Attachment {
	att := &Attachment{
		Type: "image",
	}

	transport, err := img.Decode()
	if err == nil && transport != nil {
		if ancillary := transport.GetAncillary(); ancillary != nil {
			att.Width = int(ancillary.GetWidth())
			att.Height = int(ancillary.GetHeight())
		}
		if integral := transport.GetIntegral(); integral != nil {
			if waTransport := integral.GetTransport(); waTransport != nil {
				if ancillary := waTransport.GetAncillary(); ancillary != nil {
					att.MimeType = ancillary.GetMimetype()
					att.FileSize = int64(ancillary.GetFileLength())
				}
				if integral := waTransport.GetIntegral(); integral != nil {
					att.MediaKey = integral.GetMediaKey()
					att.MediaSHA256 = integral.GetFileSHA256()
					att.MediaEncSHA256 = integral.GetFileEncSHA256()
					if integral.DirectPath != nil {
						att.DirectPath = *integral.DirectPath
					}
				}
			}
		}
	}

	return att
}

// extractE2EEVideoAttachment extracts video attachment metadata
func (c *Client) extractE2EEVideoAttachment(vid *waConsumerApplication.ConsumerApplication_VideoMessage) *Attachment {
	att := &Attachment{
		Type: "video",
	}

	transport, err := vid.Decode()
	if err == nil && transport != nil {
		if ancillary := transport.GetAncillary(); ancillary != nil {
			att.Width = int(ancillary.GetWidth())
			att.Height = int(ancillary.GetHeight())
			att.Duration = int(ancillary.GetSeconds())
		}
		if integral := transport.GetIntegral(); integral != nil {
			if waTransport := integral.GetTransport(); waTransport != nil {
				if ancillary := waTransport.GetAncillary(); ancillary != nil {
					att.MimeType = ancillary.GetMimetype()
					att.FileSize = int64(ancillary.GetFileLength())
				}
				if integral := waTransport.GetIntegral(); integral != nil {
					att.MediaKey = integral.GetMediaKey()
					att.MediaSHA256 = integral.GetFileSHA256()
					att.MediaEncSHA256 = integral.GetFileEncSHA256()
					if integral.DirectPath != nil {
						att.DirectPath = *integral.DirectPath
					}
				}
			}
		}
	}

	return att
}

// extractE2EEAudioAttachment extracts audio attachment metadata
func (c *Client) extractE2EEAudioAttachment(audio *waConsumerApplication.ConsumerApplication_AudioMessage) *Attachment {
	att := &Attachment{
		Type: "voice",
	}

	if !audio.GetPTT() {
		att.Type = "audio"
	}

	transport, err := audio.Decode()
	if err == nil && transport != nil {
		if ancillary := transport.GetAncillary(); ancillary != nil {
			att.Duration = int(ancillary.GetSeconds())
		}
		if integral := transport.GetIntegral(); integral != nil {
			if waTransport := integral.GetTransport(); waTransport != nil {
				if ancillary := waTransport.GetAncillary(); ancillary != nil {
					att.MimeType = ancillary.GetMimetype()
					att.FileSize = int64(ancillary.GetFileLength())
				}
				if integral := waTransport.GetIntegral(); integral != nil {
					att.MediaKey = integral.GetMediaKey()
					att.MediaSHA256 = integral.GetFileSHA256()
					att.MediaEncSHA256 = integral.GetFileEncSHA256()
					if integral.DirectPath != nil {
						att.DirectPath = *integral.DirectPath
					}
				}
			}
		}
	}

	return att
}

// extractE2EEDocumentAttachment extracts document attachment metadata
func (c *Client) extractE2EEDocumentAttachment(doc *waConsumerApplication.ConsumerApplication_DocumentMessage) *Attachment {
	att := &Attachment{
		Type:     "file",
		FileName: doc.GetFileName(),
	}

	transport, err := doc.Decode()
	if err == nil && transport != nil {
		if integral := transport.GetIntegral(); integral != nil {
			if waTransport := integral.GetTransport(); waTransport != nil {
				if ancillary := waTransport.GetAncillary(); ancillary != nil {
					att.MimeType = ancillary.GetMimetype()
					att.FileSize = int64(ancillary.GetFileLength())
				}
				if integral := waTransport.GetIntegral(); integral != nil {
					att.MediaKey = integral.GetMediaKey()
					att.MediaSHA256 = integral.GetFileSHA256()
					att.MediaEncSHA256 = integral.GetFileEncSHA256()
					if integral.DirectPath != nil {
						att.DirectPath = *integral.DirectPath
					}
				}
			}
		}
	}

	return att
}

// extractE2EEStickerAttachment extracts sticker attachment metadata
func (c *Client) extractE2EEStickerAttachment(sticker *waConsumerApplication.ConsumerApplication_StickerMessage) *Attachment {
	att := &Attachment{
		Type: "sticker",
	}

	transport, err := sticker.Decode()
	if err == nil && transport != nil {
		if ancillary := transport.GetAncillary(); ancillary != nil {
			att.Width = int(ancillary.GetWidth())
			att.Height = int(ancillary.GetHeight())
		}
		if integral := transport.GetIntegral(); integral != nil {
			if waTransport := integral.GetTransport(); waTransport != nil {
				if ancillary := waTransport.GetAncillary(); ancillary != nil {
					att.MimeType = ancillary.GetMimetype()
					att.FileSize = int64(ancillary.GetFileLength())
				}
				if integral := waTransport.GetIntegral(); integral != nil {
					att.MediaKey = integral.GetMediaKey()
					att.MediaSHA256 = integral.GetFileSHA256()
					att.MediaEncSHA256 = integral.GetFileEncSHA256()
					if integral.DirectPath != nil {
						att.DirectPath = *integral.DirectPath
					}
				}
			}
		}
	}

	return att
}

// extractURLFromLPHP extracts original URL from Facebook redirect URL
func extractURLFromLPHP(addr string) string {
	if addr == "" {
		return ""
	}
	parsed, err := url.Parse(addr)
	if err != nil {
		return addr
	}
	if parsed.Path == "/l.php" || strings.HasSuffix(parsed.Path, "/l.php") {
		if u := parsed.Query().Get("u"); u != "" {
			return u
		}
	}
	return addr
}
