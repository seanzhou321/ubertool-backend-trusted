package unit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	fcmmessaging "firebase.google.com/go/v4/messaging"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// noRetryDelays removes all back-off so tests complete instantly.
var noRetryDelays = []time.Duration{}

// threeRetries matches the production retry schedule but with zero waits.
var threeZeroDelays = []time.Duration{0, 0, 0}

// errTransient is a generic transient FCM error (IsUnregistered = false).
var errTransient = errors.New("internal-error: service unavailable")

// isUnregisteredByMarker identifies a "fake unregistered" error used in tests.
func isUnregisteredByMarker(err error) bool {
	return err != nil && err.Error() == "registration-token-not-registered"
}

// errUnregistered simulates a Firebase UNREGISTERED error for testing.
var errUnregistered = errors.New("registration-token-not-registered")

// newPushSvc builds a PushNotificationService wired for synchronous testing.
func newPushSvc(sender service.FCMSender, fcmRepo *MockFcmTokenRepo, delays []time.Duration) service.PushNotificationService {
	return service.NewPushNotificationServiceForTest(sender, fcmRepo, delays, isUnregisteredByMarker)
}

// --------------------------------------------------------------------------
// SendToUser: gateway / routing checks
// --------------------------------------------------------------------------

func TestPushSvc_SendToUser_NilFCMSender(t *testing.T) {
	// When no FCM client is wired (e.g. dev env without credentials), SendToUser must be
	// a no-op and must NOT touch the token repo at all.
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(nil, fcmRepo, noRetryDelays)

	err := svc.SendToUser(context.Background(), 1, "Hello", "World", 42, nil)

	assert.NoError(t, err)
	fcmRepo.AssertNotCalled(t, "GetActiveByUserID", mock.Anything, mock.Anything)
}

func TestPushSvc_SendToUser_TokenRepoError(t *testing.T) {
	// When GetActiveByUserID returns an error, SendToUser must propagate it.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, noRetryDelays)

	repoErr := errors.New("db connection lost")
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(7)).Return(nil, repoErr)

	err := svc.SendToUser(context.Background(), 7, "Hi", "Body", 1, nil)

	assert.ErrorIs(t, err, repoErr)
	sender.AssertNotCalled(t, "Send", mock.Anything, mock.Anything)
}

func TestPushSvc_SendToUser_NoActiveTokens(t *testing.T) {
	// When a user has no active tokens, send must be skipped silently.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, noRetryDelays)

	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(5)).Return([]domain.FcmToken{}, nil)

	err := svc.SendToUser(context.Background(), 5, "Title", "Msg", 10, nil)

	assert.NoError(t, err)
	sender.AssertNotCalled(t, "Send", mock.Anything, mock.Anything)
}

func TestPushSvc_SendToUser_SingleToken_Success(t *testing.T) {
	// Happy path: one active token → Send called exactly once with the right token.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, noRetryDelays)

	token := domain.FcmToken{UserID: 3, Token: "tok-aaa"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(3)).Return([]domain.FcmToken{token}, nil)
	sender.On("Send", mock.Anything, mock.MatchedBy(func(msg *fcmmessaging.Message) bool {
		return msg.Token == "tok-aaa" && msg.Data["title"] == "New Rental"
	})).Return("msg-id-1", nil)

	err := svc.SendToUser(context.Background(), 3, "New Rental", "You have a request", 99, map[string]string{"type": "RENTAL_REQUEST"})

	assert.NoError(t, err)
	sender.AssertExpectations(t)
}

func TestPushSvc_SendToUser_MultipleTokens_AllCalled(t *testing.T) {
	// Each active token must receive an independent Send call.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, noRetryDelays)

	tokens := []domain.FcmToken{
		{UserID: 8, Token: "tok-device-A"},
		{UserID: 8, Token: "tok-device-B"},
		{UserID: 8, Token: "tok-device-C"},
	}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(8)).Return(tokens, nil)
	sender.On("Send", mock.Anything, mock.MatchedBy(func(m *fcmmessaging.Message) bool {
		return m.Token == "tok-device-A"
	})).Return("id-A", nil)
	sender.On("Send", mock.Anything, mock.MatchedBy(func(m *fcmmessaging.Message) bool {
		return m.Token == "tok-device-B"
	})).Return("id-B", nil)
	sender.On("Send", mock.Anything, mock.MatchedBy(func(m *fcmmessaging.Message) bool {
		return m.Token == "tok-device-C"
	})).Return("id-C", nil)

	err := svc.SendToUser(context.Background(), 8, "Alert", "Body", 55, nil)

	assert.NoError(t, err)
	sender.AssertExpectations(t)
}

func TestPushSvc_SendToUser_PayloadContainsNotificationID(t *testing.T) {
	// The data payload must always include notification_id so the client can report events.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, noRetryDelays)

	token := domain.FcmToken{UserID: 2, Token: "tok-x"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(2)).Return([]domain.FcmToken{token}, nil)
	sender.On("Send", mock.Anything, mock.MatchedBy(func(msg *fcmmessaging.Message) bool {
		return msg.Data["notification_id"] == fmt.Sprintf("%d", int64(77)) &&
			msg.Data["type"] == "RENTAL_APPROVED"
	})).Return("msg-id", nil)

	err := svc.SendToUser(context.Background(), 2, "Approved", "Your rental is approved", 77,
		map[string]string{"type": "RENTAL_APPROVED"})

	assert.NoError(t, err)
	sender.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// sendAsync: retry and permanent-failure paths
// --------------------------------------------------------------------------

func TestPushSvc_SendAsync_SuccessFirstAttempt(t *testing.T) {
	// Send succeeds immediately → only one call to FCMSender.Send.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, threeZeroDelays)

	token := domain.FcmToken{UserID: 1, Token: "tok-ok"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(1)).Return([]domain.FcmToken{token}, nil)
	sender.On("Send", mock.Anything, mock.Anything).Return("id", nil).Once()

	err := svc.SendToUser(context.Background(), 1, "T", "B", 1, nil)
	assert.NoError(t, err)
	sender.AssertExpectations(t) // exactly one call
}

func TestPushSvc_SendAsync_SuccessOnSecondAttempt(t *testing.T) {
	// First attempt fails transiently; second attempt succeeds.
	// Send must be called exactly twice.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, threeZeroDelays)

	token := domain.FcmToken{UserID: 4, Token: "tok-retry"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(4)).Return([]domain.FcmToken{token}, nil)
	sender.On("Send", mock.Anything, mock.Anything).Return("", errTransient).Once()
	sender.On("Send", mock.Anything, mock.Anything).Return("ok", nil).Once()

	err := svc.SendToUser(context.Background(), 4, "T", "B", 2, nil)
	assert.NoError(t, err)
	sender.AssertExpectations(t)
}

func TestPushSvc_SendAsync_AllRetriesExhausted(t *testing.T) {
	// All 1 + len(retryDelays) attempts fail → send is called exactly 4 times (1 initial + 3 retries).
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, threeZeroDelays)

	token := domain.FcmToken{UserID: 6, Token: "tok-fail"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(6)).Return([]domain.FcmToken{token}, nil)
	// Fail on every attempt — allow any number of calls, then assert exact count.
	sender.On("Send", mock.Anything, mock.Anything).Return("", errTransient)

	_ = svc.SendToUser(context.Background(), 6, "T", "B", 3, nil)

	totalExpected := 1 + len(threeZeroDelays) // 4
	sender.AssertNumberOfCalls(t, "Send", totalExpected)
	fcmRepo.AssertNotCalled(t, "MarkObsolete", mock.Anything, mock.Anything)
}

func TestPushSvc_SendAsync_UnregisteredError_MarksObsolete_NoRetry(t *testing.T) {
	// When FCM returns a registration-not-registered error the token must be marked obsolete
	// and Send must NOT be retried.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, threeZeroDelays)

	token := domain.FcmToken{UserID: 9, Token: "tok-dead"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(9)).Return([]domain.FcmToken{token}, nil)
	sender.On("Send", mock.Anything, mock.Anything).Return("", errUnregistered).Once()
	fcmRepo.On("MarkObsolete", mock.Anything, "tok-dead").Return(nil).Once()

	_ = svc.SendToUser(context.Background(), 9, "T", "B", 4, nil)

	sender.AssertExpectations(t)
	fcmRepo.AssertExpectations(t)
}

func TestPushSvc_SendAsync_UnregisteredError_MarkObsoleteFails_NoRetry(t *testing.T) {
	// Even when MarkObsolete fails, Send must still not be retried.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, threeZeroDelays)

	token := domain.FcmToken{UserID: 11, Token: "tok-gone"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(11)).Return([]domain.FcmToken{token}, nil)
	sender.On("Send", mock.Anything, mock.Anything).Return("", errUnregistered).Once()
	fcmRepo.On("MarkObsolete", mock.Anything, "tok-gone").Return(errors.New("db error")).Once()

	_ = svc.SendToUser(context.Background(), 11, "T", "B", 5, nil)

	sender.AssertNumberOfCalls(t, "Send", 1)
	fcmRepo.AssertExpectations(t)
}

func TestPushSvc_SendAsync_TransientThenUnregistered_StopsAtUnregistered(t *testing.T) {
	// First attempt: transient error → retry.
	// Second attempt: unregistered → mark obsolete, stop retrying.
	// Total Send calls: 2.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, threeZeroDelays)

	token := domain.FcmToken{UserID: 14, Token: "tok-mix"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(14)).Return([]domain.FcmToken{token}, nil)
	sender.On("Send", mock.Anything, mock.Anything).Return("", errTransient).Once()
	sender.On("Send", mock.Anything, mock.Anything).Return("", errUnregistered).Once()
	fcmRepo.On("MarkObsolete", mock.Anything, "tok-mix").Return(nil).Once()

	_ = svc.SendToUser(context.Background(), 14, "T", "B", 6, nil)

	sender.AssertNumberOfCalls(t, "Send", 2)
	fcmRepo.AssertExpectations(t)
}

func TestPushSvc_SendToUser_ZeroRetries_FailsOnce(t *testing.T) {
	// With an empty retryDelays slice, exactly one attempt must be made.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)
	svc := newPushSvc(sender, fcmRepo, noRetryDelays) // zero retries

	token := domain.FcmToken{UserID: 20, Token: "tok-once"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(20)).Return([]domain.FcmToken{token}, nil)
	sender.On("Send", mock.Anything, mock.Anything).Return("", errTransient)

	_ = svc.SendToUser(context.Background(), 20, "T", "B", 7, nil)

	sender.AssertNumberOfCalls(t, "Send", 1)
}

// --------------------------------------------------------------------------
// Retry timing sanity check (zero delays, but validates attempt order)
// --------------------------------------------------------------------------

func TestPushSvc_RetryAttemptOrder(t *testing.T) {
	// Capture the order of Send calls to confirm attempt numbering is correct.
	sender := new(MockFCMSender)
	fcmRepo := new(MockFcmTokenRepo)

	delays := []time.Duration{0, 0} // two retries, no wait
	svc := newPushSvc(sender, fcmRepo, delays)

	token := domain.FcmToken{UserID: 30, Token: "tok-order"}
	fcmRepo.On("GetActiveByUserID", mock.Anything, int32(30)).Return([]domain.FcmToken{token}, nil)

	callCount := 0
	sender.On("Send", mock.Anything, mock.Anything).Return("", errTransient).Times(3).
		Run(func(args mock.Arguments) { callCount++ })

	_ = svc.SendToUser(context.Background(), 30, "T", "B", 8, nil)

	// 1 initial + 2 retries = 3 calls
	assert.Equal(t, 3, callCount)
}
