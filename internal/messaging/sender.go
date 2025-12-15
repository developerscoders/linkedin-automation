package messaging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"linkedin-automation/internal/stealth"
	"linkedin-automation/internal/storage"
	"linkedin-automation/pkg/logger"

	"github.com/go-rod/rod"
)

type Sender struct {
	page      *rod.Page
	templates *TemplateEngine
	// stealth *stealth.StealthEngine // Removed undefined
	// Again, individual components
	typer *stealth.Typer
	mouse *stealth.Mouse

	storage *storage.DB
	logger  logger.Logger
}

func NewSender(page *rod.Page, typer *stealth.Typer, mouse *stealth.Mouse, storage *storage.DB, logger logger.Logger) *Sender {
	return &Sender{
		page:      page,
		templates: NewTemplateEngine(),
		typer:     typer,
		mouse:     mouse,
		storage:   storage,
		logger:    logger,
	}
}

func (s *Sender) AddTemplate(name, content string) error {
	return s.templates.AddTemplate(name, content)
}

func (s *Sender) SendMessage(ctx context.Context, profile storage.Profile, templateName string) error {
	// 1. Render message
	data := map[string]string{
		"Name":    profile.Name,
		"Company": profile.Company,
		"Title":   profile.Title,
	}
	content, err := s.templates.Render(templateName, data)
	if err != nil {
		return err
	}

	// 2. Navigate to Message (or profile then message)
	// Easiest is to go to profile and click Message
	s.page.MustNavigate(profile.URL)
	s.page.MustWaitLoad()
	time.Sleep(3 * time.Second) // Think time

	// 3. Click Message button
	// Assuming connected, "Message" should be primary or secondary
	msgBtn, err := s.page.ElementR("button", "Message")
	if err != nil {
		return fmt.Errorf("message button not found (not connected?)")
	}
	msgBtn.MustClick()

	// 4. Wait for chat window
	time.Sleep(2 * time.Second)

	// 5. Focus Input
	inputElem, err := s.page.Element(".msg-form__contenteditable")
	if err != nil {
		return fmt.Errorf("chat input not found")
	}
	inputElem.MustClick()

	// 6. Type Message
	if err := s.typer.TypeHumanLike(inputElem, content, 0); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	// 7. Click Send
	sendBtn, err := s.page.Element(".msg-form__send-button")
	if err != nil {
		return fmt.Errorf("send button not found")
	}
	sendBtn.MustClick()

	// 8. Track
	s.storage.SaveMessage(ctx, storage.Message{
		ProfileID: profile.ID.Hex(), // Use Hex for ID string if using ObjectID, or keep original ID string logic
		// Wait, Profile.ID is ObjectID in new model. ProfileID in message is string.
		// If Profile struct uses primitive.ObjectID, we need to convert.
		// Assuming Profile passed here is the NEW struct which has ID as ObjectID.
		// But wait, "profile storage.Profile" argument might be old struct if I didn't update imports or definition!
		// 'internal/storage' is imported. The struct is updated.
		// So profile.ID is primitive.ObjectID.
		// Message.ProfileID is string.
		// So profile.ID.Hex() is correct.
		Content:      content,
		TemplateName: templateName,
		Status:       "sent",
	})

	return nil
}

func (s *Sender) MessageFromCard(ctx context.Context, card *rod.Element, profile *storage.Profile, templateName string) error {
	// 1. Render message
	data := map[string]string{
		"Name":    profile.Name,
		"Company": profile.Company,
		"Title":   profile.Title,
	}
	content, err := s.templates.Render(templateName, data)
	if err != nil {
		return err
	}

	// 2. Find Message button on card
	// Selector provided: button[aria-label='Message <Name>'] class='artdeco-button ...'

	findMessageBtn := func() (*rod.Element, error) {
		// 1. Precise Aria Label
		// button[aria-label^='Message']
		if btn, err := card.Element("button[aria-label^='Message']"); err == nil {
			return btn, nil
		}

		// 2. Artdeco Button with "Message" text
		if btns, err := card.Elements(".artdeco-button"); err == nil {
			for _, btn := range btns {
				if txt, err := btn.Text(); err == nil && strings.TrimSpace(txt) == "Message" {
					return btn, nil
				}
			}
		}

		// 3. Fallback
		return card.ElementR("button, a", "Message")
	}

	msgBtn, err := findMessageBtn()
	if err != nil {
		return fmt.Errorf("message button not found on card")
	}

	// 3. Click Message
	// Stealth move
	box := msgBtn.MustShape().Box()
	s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
	msgBtn.MustClick()
	time.Sleep(2 * time.Second)

	// 4. Wait for chat overlay
	// The overlay usually pops up at the bottom right.
	// We need to find the active chat window, often has class "msg-overlay-conversation-bubble" usually.
	// Or look for the input field directly which should now be visible.

	// Wait for input
	inputElem, err := s.page.Element(".msg-form__contenteditable")
	if err != nil {
		return fmt.Errorf("chat input not found after clicking message")
	}

	// Check if we need to click inside it?
	inputElem.MustClick()
	time.Sleep(500 * time.Millisecond)

	// 5. Type Message
	if err := s.typer.TypeHumanLike(inputElem, content, 0); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	// 6. Click Send
	sendBtn, err := s.page.Element(".msg-form__send-button")
	if err != nil {
		return fmt.Errorf("send button not found in chat")
	}

	// Check if enabled
	if disabled, _ := sendBtn.Attribute("disabled"); disabled != nil {
		// Try to see if there is text?
		return fmt.Errorf("send button disabled")
	}

	sBtnBox := sendBtn.MustShape().Box()
	s.mouse.MoveTo(s.page, stealth.Point{X: sBtnBox.X + sBtnBox.Width/2, Y: sBtnBox.Y + sBtnBox.Height/2})
	sendBtn.MustClick()

	s.logger.Info("message sent from card", "profile", profile.Name)

	// 7. Track
	s.storage.SaveMessage(ctx, storage.Message{
		ProfileID:    profile.ID.Hex(),
		Content:      content,
		TemplateName: templateName,
		Status:       "sent",
		SentAt:       time.Now(),
	})

	// 8. Close chat window (optional)
	// Find close button for this conversation
	// usually header button with aria-label="Close your conversation with ..." or just generic close icon
	// .msg-overlay-bubble-header__control--close-btn
	if closeBtn, err := s.page.Element("button[data-control-name='overlay.close_conversation_window']"); err == nil {
		closeBtn.MustClick()
	} else if closeBtn, err := s.page.Element(".msg-overlay-bubble-header__control--close-btn"); err == nil {
		closeBtn.MustClick()
	}

	return nil
}
