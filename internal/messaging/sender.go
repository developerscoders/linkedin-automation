package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"linkedin-automation/internal/storage"
	"linkedin-automation/internal/stealth"
	"linkedin-automation/pkg/logger"
)

type Sender struct {
	page      *rod.Page
	templates *TemplateEngine
	// stealth *stealth.StealthEngine // Removed undefined
	// Again, individual components
	typer     *stealth.Typer
	mouse     *stealth.Mouse
	
	storage   *storage.DB
	logger    logger.Logger
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
		ProfileID:    profile.ID.Hex(), // Use Hex for ID string if using ObjectID, or keep original ID string logic
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
