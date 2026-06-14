package notifications

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"watchflare/backend/encryption"
)

// Service orchestrates encrypted channel storage and notification delivery.
type Service struct {
	repo          *Repository
	notifier      Notifier
	encryptionKey string
}

func NewService(repo *Repository, notifier Notifier, encryptionKey string) *Service {
	return &Service{repo: repo, notifier: notifier, encryptionKey: encryptionKey}
}

// Broadcast sends title+message to every enabled channel that subscribes to category.
// Sends are issued concurrently; the returned slice aggregates per-channel errors.
func (s *Service) Broadcast(ctx context.Context, category, title, message string) []error {
	channels, err := s.repo.ListEnabledByCategory(ctx, category)
	if err != nil {
		return []error{err}
	}
	if len(channels) == 0 {
		return nil
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, ch := range channels {
		wg.Add(1)
		go func(ch Channel) {
			defer wg.Done()
			url, err := s.DecryptURL(ch.URLEncrypted)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("decrypt channel %s: %w", ch.ID, err))
				mu.Unlock()
				return
			}
			if err := s.notifier.Send(ctx, url, title, message); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("send to channel %s (%s): %w", ch.ID, ch.Name, err))
				mu.Unlock()
			}
		}(ch)
	}
	wg.Wait()
	return errs
}

// SendToChannel delivers a one-off message to a single channel. Used for the
// "test channel" endpoint.
func (s *Service) SendToChannel(ctx context.Context, id, title, message string) error {
	ch, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	url, err := s.DecryptURL(ch.URLEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt channel %s: %w", id, err)
	}
	return s.notifier.Send(ctx, url, title, message)
}

// Repo exposes the underlying repository so HTTP handlers can query channels
// directly without going through Broadcast.
func (s *Service) Repo() *Repository {
	return s.repo
}

// errEncryptionKeyMissing is returned whenever a method needs the encryption
// key but it was not configured. Centralized so the message stays consistent
// between EncryptURL, DecryptURL and Broadcast paths.
var errEncryptionKeyMissing = errors.New("NOTIFICATION_ENCRYPTION_KEY is not configured")

// EncryptURL encrypts a plain shoutrrr URL for storage.
func (s *Service) EncryptURL(plain string) (string, error) {
	if s.encryptionKey == "" {
		return "", errEncryptionKeyMissing
	}
	return encryption.Encrypt(plain, s.encryptionKey)
}

// DecryptURL reconstructs the plain URL from its stored ciphertext.
func (s *Service) DecryptURL(encrypted string) (string, error) {
	if s.encryptionKey == "" {
		return "", errEncryptionKeyMissing
	}
	return encryption.Decrypt(encrypted, s.encryptionKey)
}
