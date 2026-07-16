package handlers

import "watchflare/backend/services"

// notifyAccountEvent is the indirection used to emit transactional account
// notifications, overridable in tests.
var notifyAccountEvent = services.NotifyAccountEvent
