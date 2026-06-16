package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/nicholas-fedor/shoutrrr"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ShoutrrrNotifier sends notifications through the shoutrrr library.
type ShoutrrrNotifier struct{}

func NewShoutrrrNotifier() *ShoutrrrNotifier {
	return &ShoutrrrNotifier{}
}

// ValidateShoutrrrURL returns nil if url is a syntactically valid shoutrrr
// service URL (discord://, slack://, smtp://, etc.). Used at create/update time
// to reject malformed inputs before they are encrypted and stored.
func ValidateShoutrrrURL(url string) error {
	if url == "" {
		return errors.New("empty shoutrrr url")
	}
	if _, err := shoutrrr.CreateSender(url); err != nil {
		return fmt.Errorf("invalid shoutrrr url: %w", err)
	}
	return nil
}

// Send delivers title and message to the channel addressed by url.
// The url must be a valid shoutrrr service URL (discord://, slack://, smtp://, etc.).
func (n *ShoutrrrNotifier) Send(ctx context.Context, url, title, message string) error {
	if url == "" {
		return errors.New("empty shoutrrr url")
	}

	sender, err := shoutrrr.CreateSender(url)
	if err != nil {
		return fmt.Errorf("create shoutrrr sender: %w", err)
	}

	params := types.Params{}
	if title != "" {
		params["title"] = title
	}

	done := make(chan []error, 1)
	go func() {
		done <- sender.Send(message, &params)
	}()

	select {
	case errs := <-done:
		return errors.Join(errs...)
	case <-ctx.Done():
		return ctx.Err()
	}
}
