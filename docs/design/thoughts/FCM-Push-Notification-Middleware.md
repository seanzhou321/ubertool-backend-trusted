# FCM Push Notification Middleware - Design Document

## Overview

Firebase Cloud Messaging (FCM) middleware for sending push notifications to Android and iOS mobile clients. This middleware handles device token management, notification formatting, delivery, retry logic, and analytics.

---

## Architecture

```mermaid
graph TB
    subgraph "Application Services"
        RENTAL[Rental Service]
        REVIEW[Review Service]
        AUTH[Auth Service]
    end
    
    subgraph "Notification Layer"
        QUEUE[Message Queue<br/>Bull/RabbitMQ]
        WORKER[Notification Worker]
        FCM_MW[FCM Middleware]
    end
    
    subgraph "External Services"
        FCM[Firebase Cloud<br/>Messaging]
        DB[(PostgreSQL<br/>Device Tokens)]
    end
    
    subgraph "Mobile Clients"
        ANDROID[Android App]
        IOS[iOS App]
    end
    
    RENTAL -->|Publish Event| QUEUE
    REVIEW -->|Publish Event| QUEUE
    AUTH -->|Publish Event| QUEUE
    
    QUEUE -->|Consume| WORKER
    WORKER -->|Get Tokens| DB
    WORKER -->|Send Notification| FCM_MW
    
    FCM_MW -->|Validate & Format| FCM_MW
    FCM_MW -->|Send via Admin SDK| FCM
    
    FCM -->|Deliver| ANDROID
    FCM -->|Deliver| IOS
    
    FCM_MW -->|Log Result| DB
```

---

## Database Schema

### Device Tokens Table

```sql
CREATE TABLE device_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_token VARCHAR(255) NOT NULL UNIQUE,
    platform VARCHAR(10) NOT NULL CHECK (platform IN ('android', 'ios')),
    device_info JSONB,  -- Device model, OS version, app version
    is_active BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT unique_user_device UNIQUE(user_id, device_token)
);

CREATE INDEX idx_device_tokens_user_id ON device_tokens(user_id) WHERE is_active = TRUE;
CREATE INDEX idx_device_tokens_platform ON device_tokens(platform) WHERE is_active = TRUE;
CREATE INDEX idx_device_tokens_last_used ON device_tokens(last_used_at);
```

### Push Notification Log Table

```sql
CREATE TABLE push_notification_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_token_id BIGINT REFERENCES device_tokens(id) ON DELETE SET NULL,
    notification_type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    data JSONB,  -- Additional payload data
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'sent', 'failed', 'invalid_token')),
    error_message TEXT,
    fcm_message_id VARCHAR(255),  -- FCM response message ID
    retry_count INTEGER DEFAULT 0,
    sent_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT max_retries CHECK (retry_count <= 3)
);

CREATE INDEX idx_push_logs_user_id ON push_notification_logs(user_id);
CREATE INDEX idx_push_logs_status ON push_notification_logs(status);
CREATE INDEX idx_push_logs_created_at ON push_notification_logs(created_at);
CREATE INDEX idx_push_logs_type ON push_notification_logs(notification_type);

-- Partition by month for performance (optional)
-- CREATE TABLE push_notification_logs_2026_01 PARTITION OF push_notification_logs
-- FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
```

---

## FCM Middleware Implementation

### 1. FCM Service Class (TypeScript)

```typescript
// src/services/fcm/fcm.service.ts

import * as admin from 'firebase-admin';
import { Message, MulticastMessage, BatchResponse } from 'firebase-admin/messaging';
import { PrismaClient } from '@prisma/client';
import { Logger } from '../utils/logger';

export interface PushNotificationPayload {
  userId: number;
  type: string;
  title: string;
  body: string;
  data?: Record<string, string>;
  imageUrl?: string;
  priority?: 'high' | 'normal';
  ttl?: number;  // Time to live in seconds
}

export interface SendResult {
  success: boolean;
  messageId?: string;
  error?: string;
  invalidToken?: boolean;
}

export class FCMService {
  private prisma: PrismaClient;
  private logger: Logger;
  private messaging: admin.messaging.Messaging;

  constructor() {
    this.prisma = new PrismaClient();
    this.logger = new Logger('FCMService');
    
    // Initialize Firebase Admin SDK
    if (!admin.apps.length) {
      admin.initializeApp({
        credential: admin.credential.cert({
          projectId: process.env.FIREBASE_PROJECT_ID,
          clientEmail: process.env.FIREBASE_CLIENT_EMAIL,
          privateKey: process.env.FIREBASE_PRIVATE_KEY?.replace(/\\n/g, '\n'),
        }),
      });
    }
    
    this.messaging = admin.messaging();
  }

  /**
   * Send push notification to a single user (all their devices)
   */
  async sendToUser(payload: PushNotificationPayload): Promise<SendResult[]> {
    try {
      // Get all active device tokens for the user
      const deviceTokens = await this.prisma.deviceTokens.findMany({
        where: {
          userId: payload.userId,
          isActive: true,
        },
      });

      if (deviceTokens.length === 0) {
        this.logger.warn(`No active device tokens found for user ${payload.userId}`);
        return [];
      }

      // Send to all devices
      const results = await Promise.all(
        deviceTokens.map(device => 
          this.sendToDevice(device.deviceToken, device.platform, payload, device.id)
        )
      );

      return results;
    } catch (error) {
      this.logger.error('Error sending notification to user', error);
      throw error;
    }
  }

  /**
   * Send push notification to a specific device token
   */
  private async sendToDevice(
    deviceToken: string,
    platform: 'android' | 'ios',
    payload: PushNotificationPayload,
    deviceTokenId: number
  ): Promise<SendResult> {
    try {
      // Create log entry
      const log = await this.prisma.pushNotificationLogs.create({
        data: {
          userId: payload.userId,
          deviceTokenId,
          notificationType: payload.type,
          title: payload.title,
          body: payload.body,
          data: payload.data || {},
          status: 'pending',
        },
      });

      // Build FCM message
      const message = this.buildMessage(deviceToken, platform, payload);

      // Send via FCM
      const response = await this.messaging.send(message);

      // Update log with success
      await this.prisma.pushNotificationLogs.update({
        where: { id: log.id },
        data: {
          status: 'sent',
          fcmMessageId: response,
          sentAt: new Date(),
        },
      });

      // Update device token last used
      await this.prisma.deviceTokens.update({
        where: { id: deviceTokenId },
        data: { lastUsedAt: new Date() },
      });

      this.logger.info(`Push notification sent successfully: ${response}`);

      return {
        success: true,
        messageId: response,
      };
    } catch (error: any) {
      return await this.handleSendError(error, deviceToken, deviceTokenId, payload);
    }
  }

  /**
   * Send push notification to multiple users (batch)
   */
  async sendToMultipleUsers(
    userIds: number[],
    payload: Omit<PushNotificationPayload, 'userId'>
  ): Promise<Map<number, SendResult[]>> {
    const results = new Map<number, SendResult[]>();

    for (const userId of userIds) {
      const userResults = await this.sendToUser({
        ...payload,
        userId,
      });
      results.set(userId, userResults);
    }

    return results;
  }

  /**
   * Build FCM message with platform-specific configuration
   */
  private buildMessage(
    deviceToken: string,
    platform: 'android' | 'ios',
    payload: PushNotificationPayload
  ): Message {
    const message: Message = {
      token: deviceToken,
      notification: {
        title: payload.title,
        body: payload.body,
        imageUrl: payload.imageUrl,
      },
      data: {
        type: payload.type,
        ...payload.data,
      },
    };

    // Platform-specific configuration
    if (platform === 'android') {
      message.android = {
        priority: payload.priority || 'high',
        ttl: payload.ttl || 86400000, // 24 hours default
        notification: {
          channelId: this.getAndroidChannelId(payload.type),
          priority: payload.priority === 'high' ? 'high' : 'default',
          sound: 'default',
          clickAction: 'FLUTTER_NOTIFICATION_CLICK',
          color: '#2D7A3E', // Ubertool brand color
        },
      };
    } else if (platform === 'ios') {
      message.apns = {
        headers: {
          'apns-priority': payload.priority === 'high' ? '10' : '5',
          'apns-expiration': String(Math.floor(Date.now() / 1000) + (payload.ttl || 86400)),
        },
        payload: {
          aps: {
            alert: {
              title: payload.title,
              body: payload.body,
            },
            sound: 'default',
            badge: 1,
            category: payload.type,
            'mutable-content': 1, // For rich notifications
          },
        },
      };
    }

    return message;
  }

  /**
   * Get Android notification channel ID based on notification type
   */
  private getAndroidChannelId(type: string): string {
    const channelMap: Record<string, string> = {
      rental_request_received: 'rental_requests',
      rental_request_accepted: 'rental_updates',
      rental_request_rejected: 'rental_updates',
      rental_request_finalized: 'rental_updates',
      rental_request_cancelled: 'rental_updates',
      review_received: 'reviews',
      email_verification: 'account',
      password_reset: 'account',
    };

    return channelMap[type] || 'default';
  }

  /**
   * Handle FCM send errors
   */
  private async handleSendError(
    error: any,
    deviceToken: string,
    deviceTokenId: number,
    payload: PushNotificationPayload
  ): Promise<SendResult> {
    this.logger.error('FCM send error', error);

    let status: 'failed' | 'invalid_token' = 'failed';
    let invalidToken = false;

    // Check for invalid token errors
    if (
      error.code === 'messaging/invalid-registration-token' ||
      error.code === 'messaging/registration-token-not-registered'
    ) {
      status = 'invalid_token';
      invalidToken = true;

      // Deactivate invalid token
      await this.prisma.deviceTokens.update({
        where: { id: deviceTokenId },
        data: { isActive: false },
      });

      this.logger.warn(`Deactivated invalid device token: ${deviceToken}`);
    }

    // Log the error
    await this.prisma.pushNotificationLogs.create({
      data: {
        userId: payload.userId,
        deviceTokenId,
        notificationType: payload.type,
        title: payload.title,
        body: payload.body,
        data: payload.data || {},
        status,
        errorMessage: error.message,
      },
    });

    return {
      success: false,
      error: error.message,
      invalidToken,
    };
  }

  /**
   * Register a new device token
   */
  async registerDeviceToken(
    userId: number,
    deviceToken: string,
    platform: 'android' | 'ios',
    deviceInfo?: Record<string, any>
  ): Promise<void> {
    try {
      await this.prisma.deviceTokens.upsert({
        where: {
          deviceToken,
        },
        update: {
          userId,
          platform,
          deviceInfo,
          isActive: true,
          lastUsedAt: new Date(),
          updatedAt: new Date(),
        },
        create: {
          userId,
          deviceToken,
          platform,
          deviceInfo,
          isActive: true,
        },
      });

      this.logger.info(`Device token registered for user ${userId}`);
    } catch (error) {
      this.logger.error('Error registering device token', error);
      throw error;
    }
  }

  /**
   * Unregister a device token (on logout)
   */
  async unregisterDeviceToken(deviceToken: string): Promise<void> {
    try {
      await this.prisma.deviceTokens.update({
        where: { deviceToken },
        data: { isActive: false },
      });

      this.logger.info(`Device token unregistered: ${deviceToken}`);
    } catch (error) {
      this.logger.error('Error unregistering device token', error);
      throw error;
    }
  }

  /**
   * Clean up old inactive tokens (run periodically)
   */
  async cleanupInactiveTokens(daysOld: number = 90): Promise<number> {
    try {
      const cutoffDate = new Date();
      cutoffDate.setDate(cutoffDate.getDate() - daysOld);

      const result = await this.prisma.deviceTokens.deleteMany({
        where: {
          isActive: false,
          updatedAt: {
            lt: cutoffDate,
          },
        },
      });

      this.logger.info(`Cleaned up ${result.count} inactive device tokens`);
      return result.count;
    } catch (error) {
      this.logger.error('Error cleaning up inactive tokens', error);
      throw error;
    }
  }

  /**
   * Get notification statistics for a user
   */
  async getUserNotificationStats(userId: number, days: number = 30) {
    const cutoffDate = new Date();
    cutoffDate.setDate(cutoffDate.getDate() - days);

    const stats = await this.prisma.pushNotificationLogs.groupBy({
      by: ['status'],
      where: {
        userId,
        createdAt: {
          gte: cutoffDate,
        },
      },
      _count: true,
    });

    return {
      period: { days, from: cutoffDate, to: new Date() },
      stats: stats.reduce((acc, stat) => {
        acc[stat.status] = stat._count;
        return acc;
      }, {} as Record<string, number>),
    };
  }
}
```

---

## 2. Notification Templates

```typescript
// src/services/fcm/notification-templates.ts

export interface NotificationTemplate {
  title: string;
  body: string;
  data?: Record<string, string>;
  imageUrl?: string;
  priority?: 'high' | 'normal';
}

export class NotificationTemplates {
  /**
   * Rental request received (to owner)
   */
  static rentalRequestReceived(
    renterName: string,
    toolName: string,
    startDate: string,
    endDate: string,
    requestId: number
  ): NotificationTemplate {
    return {
      title: '🔔 New Rental Request',
      body: `${renterName} wants to rent your ${toolName} from ${startDate} to ${endDate}`,
      data: {
        screen: 'rental_request_detail',
        requestId: String(requestId),
      },
      priority: 'high',
    };
  }

  /**
   * Rental request accepted (to renter)
   */
  static rentalRequestAccepted(
    ownerName: string,
    toolName: string,
    requestId: number
  ): NotificationTemplate {
    return {
      title: '✅ Request Accepted!',
      body: `${ownerName} accepted your request for ${toolName}. Check pickup details.`,
      data: {
        screen: 'rental_request_detail',
        requestId: String(requestId),
      },
      priority: 'high',
    };
  }

  /**
   * Rental request rejected (to renter)
   */
  static rentalRequestRejected(
    ownerName: string,
    toolName: string,
    requestId: number
  ): NotificationTemplate {
    return {
      title: '❌ Request Declined',
      body: `${ownerName} declined your request for ${toolName}`,
      data: {
        screen: 'rental_request_detail',
        requestId: String(requestId),
      },
      priority: 'normal',
    };
  }

  /**
   * Rental request finalized (to owner)
   */
  static rentalRequestFinalized(
    renterName: string,
    toolName: string,
    requestId: number
  ): NotificationTemplate {
    return {
      title: '🎉 Rental Confirmed',
      body: `${renterName} confirmed the rental of your ${toolName}`,
      data: {
        screen: 'rental_request_detail',
        requestId: String(requestId),
      },
      priority: 'high',
    };
  }

  /**
   * Rental request cancelled
   */
  static rentalRequestCancelled(
    userName: string,
    toolName: string,
    requestId: number
  ): NotificationTemplate {
    return {
      title: '🚫 Rental Cancelled',
      body: `${userName} cancelled the rental request for ${toolName}`,
      data: {
        screen: 'rental_request_detail',
        requestId: String(requestId),
      },
      priority: 'normal',
    };
  }

  /**
   * Review received
   */
  static reviewReceived(
    reviewerName: string,
    rating: number,
    reviewId: number
  ): NotificationTemplate {
    const stars = '⭐'.repeat(rating);
    return {
      title: '⭐ New Review',
      body: `${reviewerName} left you a ${rating}-star review ${stars}`,
      data: {
        screen: 'review_detail',
        reviewId: String(reviewId),
      },
      priority: 'normal',
    };
  }

  /**
   * Email verification
   */
  static emailVerification(): NotificationTemplate {
    return {
      title: '📧 Verify Your Email',
      body: 'Please verify your email address to complete registration',
      data: {
        screen: 'email_verification',
      },
      priority: 'high',
    };
  }

  /**
   * Password reset
   */
  static passwordReset(): NotificationTemplate {
    return {
      title: '🔐 Password Reset',
      body: 'A password reset was requested for your account',
      data: {
        screen: 'password_reset',
      },
      priority: 'high',
    };
  }
}
```

---

## 3. Notification Worker (Queue Consumer)

```typescript
// src/workers/notification.worker.ts

import { Job, Queue, Worker } from 'bullmq';
import { FCMService } from '../services/fcm/fcm.service';
import { NotificationTemplates } from '../services/fcm/notification-templates';
import { Logger } from '../utils/logger';

interface NotificationJob {
  type: string;
  userId: number;
  data: Record<string, any>;
}

export class NotificationWorker {
  private fcmService: FCMService;
  private logger: Logger;
  private worker: Worker;

  constructor(private queue: Queue) {
    this.fcmService = new FCMService();
    this.logger = new Logger('NotificationWorker');
    
    this.worker = new Worker(
      'notifications',
      async (job: Job<NotificationJob>) => {
        await this.processNotification(job);
      },
      {
        connection: {
          host: process.env.REDIS_HOST || 'localhost',
          port: parseInt(process.env.REDIS_PORT || '6379'),
        },
        concurrency: 10, // Process 10 notifications concurrently
      }
    );

    this.worker.on('completed', (job) => {
      this.logger.info(`Notification job ${job.id} completed`);
    });

    this.worker.on('failed', (job, err) => {
      this.logger.error(`Notification job ${job?.id} failed`, err);
    });
  }

  private async processNotification(job: Job<NotificationJob>): Promise<void> {
    const { type, userId, data } = job.data;

    this.logger.info(`Processing notification: ${type} for user ${userId}`);

    let template;

    switch (type) {
      case 'rental_request_received':
        template = NotificationTemplates.rentalRequestReceived(
          data.renterName,
          data.toolName,
          data.startDate,
          data.endDate,
          data.requestId
        );
        break;

      case 'rental_request_accepted':
        template = NotificationTemplates.rentalRequestAccepted(
          data.ownerName,
          data.toolName,
          data.requestId
        );
        break;

      case 'rental_request_rejected':
        template = NotificationTemplates.rentalRequestRejected(
          data.ownerName,
          data.toolName,
          data.requestId
        );
        break;

      case 'rental_request_finalized':
        template = NotificationTemplates.rentalRequestFinalized(
          data.renterName,
          data.toolName,
          data.requestId
        );
        break;

      case 'rental_request_cancelled':
        template = NotificationTemplates.rentalRequestCancelled(
          data.userName,
          data.toolName,
          data.requestId
        );
        break;

      case 'review_received':
        template = NotificationTemplates.reviewReceived(
          data.reviewerName,
          data.rating,
          data.reviewId
        );
        break;

      case 'email_verification':
        template = NotificationTemplates.emailVerification();
        break;

      case 'password_reset':
        template = NotificationTemplates.passwordReset();
        break;

      default:
        this.logger.warn(`Unknown notification type: ${type}`);
        return;
    }

    // Send push notification
    await this.fcmService.sendToUser({
      userId,
      type,
      ...template,
    });
  }

  async close(): Promise<void> {
    await this.worker.close();
  }
}
```

---

## 4. gRPC Service Implementation

```typescript
// src/grpc/services/notification.service.ts

import { ServerUnaryCall, sendUnaryData } from '@grpc/grpc-js';
import { FCMService } from '../../services/fcm/fcm.service';
import {
  RegisterPushTokenRequest,
  RegisterPushTokenResponse,
} from '../../generated/ubertool';

export class NotificationServiceImpl {
  private fcmService: FCMService;

  constructor() {
    this.fcmService = new FCMService();
  }

  async registerPushToken(
    call: ServerUnaryCall<RegisterPushTokenRequest, RegisterPushTokenResponse>,
    callback: sendUnaryData<RegisterPushTokenResponse>
  ): Promise<void> {
    try {
      const { deviceToken, platform } = call.request;
      const userId = call.metadata.get('user_id')[0] as number; // From JWT

      // Validate platform
      if (platform !== 'android' && platform !== 'ios') {
        callback({
          code: 3, // INVALID_ARGUMENT
          message: 'Platform must be either "android" or "ios"',
        });
        return;
      }

      // Register device token
      await this.fcmService.registerDeviceToken(
        userId,
        deviceToken,
        platform as 'android' | 'ios'
      );

      callback(null, { success: true });
    } catch (error: any) {
      callback({
        code: 13, // INTERNAL
        message: error.message,
      });
    }
  }
}
```

---

## 5. Environment Configuration

```env
# .env

# Firebase Admin SDK
FIREBASE_PROJECT_ID=ubertool-prod
FIREBASE_CLIENT_EMAIL=firebase-adminsdk@ubertool-prod.iam.gserviceaccount.com
FIREBASE_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n"

# Redis (for Bull queue)
REDIS_HOST=localhost
REDIS_PORT=6379

# Database
DATABASE_URL=postgresql://user:password@localhost:5432/ubertool
```

---

## 6. Android Notification Channels Setup

```kotlin
// Android app: NotificationChannelManager.kt

object NotificationChannelManager {
    fun createNotificationChannels(context: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channels = listOf(
                NotificationChannel(
                    "rental_requests",
                    "Rental Requests",
                    NotificationManager.IMPORTANCE_HIGH
                ).apply {
                    description = "New rental requests for your tools"
                    enableVibration(true)
                    enableLights(true)
                },
                NotificationChannel(
                    "rental_updates",
                    "Rental Updates",
                    NotificationManager.IMPORTANCE_HIGH
                ).apply {
                    description = "Updates on your rental requests"
                },
                NotificationChannel(
                    "reviews",
                    "Reviews",
                    NotificationManager.IMPORTANCE_DEFAULT
                ).apply {
                    description = "New reviews and ratings"
                },
                NotificationChannel(
                    "account",
                    "Account",
                    NotificationManager.IMPORTANCE_HIGH
                ).apply {
                    description = "Account-related notifications"
                }
            )

            val notificationManager = context.getSystemService(NotificationManager::class.java)
            channels.forEach { notificationManager.createNotificationChannel(it) }
        }
    }
}
```

---

## 7. iOS Push Notification Setup

```swift
// iOS app: NotificationManager.swift

import UserNotifications
import FirebaseMessaging

class NotificationManager: NSObject, UNUserNotificationCenterDelegate, MessagingDelegate {
    
    static let shared = NotificationManager()
    
    func registerForPushNotifications() {
        UNUserNotificationCenter.current().delegate = self
        
        let authOptions: UNAuthorizationOptions = [.alert, .badge, .sound]
        UNUserNotificationCenter.current().requestAuthorization(
            options: authOptions,
            completionHandler: { granted, _ in
                guard granted else { return }
                DispatchQueue.main.async {
                    UIApplication.shared.registerForRemoteNotifications()
                }
            }
        )
        
        Messaging.messaging().delegate = self
    }
    
    // Handle FCM token refresh
    func messaging(_ messaging: Messaging, didReceiveRegistrationToken fcmToken: String?) {
        guard let token = fcmToken else { return }
        
        // Send token to backend via gRPC
        NotificationService.shared.registerPushToken(
            deviceToken: token,
            platform: "ios"
        )
    }
    
    // Handle notification when app is in foreground
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([[.banner, .sound, .badge]])
    }
    
    // Handle notification tap
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let userInfo = response.notification.request.content.userInfo
        handleNotificationTap(userInfo: userInfo)
        completionHandler()
    }
    
    private func handleNotificationTap(userInfo: [AnyHashable: Any]) {
        guard let screen = userInfo["screen"] as? String else { return }
        
        // Navigate to appropriate screen
        switch screen {
        case "rental_request_detail":
            if let requestId = userInfo["requestId"] as? String {
                // Navigate to rental request detail
            }
        case "review_detail":
            if let reviewId = userInfo["reviewId"] as? String {
                // Navigate to review detail
            }
        default:
            break
        }
    }
}
```

---

## Usage Examples

### Sending Notification from Rental Service

```typescript
// src/services/rental.service.ts

import { Queue } from 'bullmq';

export class RentalService {
  private notificationQueue: Queue;

  async acceptRentalRequest(requestId: number, ownerId: number) {
    // ... business logic ...

    // Publish notification event to queue
    await this.notificationQueue.add('notification', {
      type: 'rental_request_accepted',
      userId: request.renterId,
      data: {
        ownerName: owner.name,
        toolName: tool.name,
        requestId: request.id,
      },
    });
  }
}
```

---

## Monitoring & Analytics

### Notification Metrics to Track

1. **Delivery Metrics:**
   - Total notifications sent
   - Success rate
   - Failure rate
   - Invalid token rate

2. **Engagement Metrics:**
   - Open rate (notification taps)
   - Time to open
   - Conversion rate (action taken)

3. **Performance Metrics:**
   - Queue processing time
   - FCM API latency
   - Retry attempts

### Monitoring Queries

```sql
-- Daily notification stats
SELECT 
    DATE(created_at) as date,
    notification_type,
    status,
    COUNT(*) as count
FROM push_notification_logs
WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY DATE(created_at), notification_type, status
ORDER BY date DESC, notification_type;

-- Invalid token cleanup candidates
SELECT 
    dt.id,
    dt.user_id,
    dt.device_token,
    dt.platform,
    dt.last_used_at,
    COUNT(pnl.id) as failed_count
FROM device_tokens dt
LEFT JOIN push_notification_logs pnl ON dt.id = pnl.device_token_id
WHERE pnl.status = 'invalid_token'
  AND pnl.created_at >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY dt.id
HAVING COUNT(pnl.id) >= 3;
```

---

## Best Practices

1. **Token Management:**
   - Deactivate tokens on logout
   - Clean up invalid tokens immediately
   - Periodically remove old inactive tokens

2. **Retry Logic:**
   - Max 3 retries with exponential backoff
   - Don't retry invalid token errors
   - Log all retry attempts

3. **Rate Limiting:**
   - Respect FCM rate limits (1 million messages/minute)
   - Implement batching for bulk sends
   - Use topic messaging for broadcasts

4. **Privacy:**
   - Don't include sensitive data in notification body
   - Use data payload for sensitive information
   - Encrypt data payload if needed

5. **Testing:**
   - Test on both Android and iOS
   - Test foreground and background scenarios
   - Test notification actions and deep linking

---

This FCM middleware design provides a robust, scalable solution for push notifications in the Ubertool mobile app!
