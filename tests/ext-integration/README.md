# External Integration Tests

This directory contains integration tests for external services that the Ubertool backend depends on, such as email providers (Gmail, Yahoo Mail).

## Purpose

Unlike unit tests and E2E tests which mock external dependencies, these tests verify actual integration with real external services. They are designed for manual execution to validate that the application can successfully communicate with third-party services.

## Email Service Integration Tests

### Gmail Integration (`gmail_test.go`)

Tests email sending functionality via Gmail's SMTP server.

**Prerequisites:**
1. A Gmail account
2. Enable 2-Step Verification in your Google Account
3. Generate an App Password:
   - Go to Google Account → Security → 2-Step Verification → App passwords
   - Generate a new app password for "Mail"
   - Copy the 16-character password

**Environment Variables:**
```bash
export GMAIL_SMTP_USER="your-email@gmail.com"
export GMAIL_SMTP_PASSWORD="your-16-char-app-password"
```

**Run Tests:**
```bash
go test -v ./tests/ext-integration -run TestGmail
```

**Test Coverage:**
- ✅ Send invitation emails
- ✅ Send rental request notifications
- ✅ Send rental approval notifications
- ✅ Send rental rejection notifications
- ✅ Send account status notifications
- ✅ Send admin notifications
- ✅ Send 2FA codes
- ✅ Error handling for invalid credentials

### Yahoo Mail Integration (`yahoo_test.go`)

Tests email sending functionality via Yahoo Mail's SMTP server.

**Prerequisites:**
1. A Yahoo Mail account
2. Generate an App Password:
   - Go to Yahoo Account Security → Generate app password
   - Select "Other App" and name it (e.g., "Ubertool")
   - Copy the generated password

**Environment Variables:**
```bash
export YAHOO_SMTP_USER="your-email@yahoo.com"
export YAHOO_SMTP_PASSWORD="your-app-password"
```

**Run Tests:**
```bash
go test -v ./tests/ext-integration -run TestYahoo
```

**Test Coverage:**
- ✅ Send invitation emails
- ✅ Send rental request notifications
- ✅ Send rental approval notifications
- ✅ Send rental completion notifications
- ✅ Send rental cancellation notifications
- ✅ Send rental confirmation notifications
- ✅ Send 2FA codes
- ✅ Error handling for invalid credentials
- ✅ Comparison test between Yahoo and Gmail

## Running All External Integration Tests

### Set Environment Variables (PowerShell)
```powershell
$env:GMAIL_SMTP_USER="your-email@gmail.com"
$env:GMAIL_SMTP_PASSWORD="your-gmail-app-password"
$env:YAHOO_SMTP_USER="your-email@yahoo.com"
$env:YAHOO_SMTP_PASSWORD="your-yahoo-app-password"
```

### Run All Tests
```bash
go test -v ./tests/ext-integration/...
```

### Run Specific Provider
```bash
# Gmail only
go test -v ./tests/ext-integration -run TestGmail

# Yahoo only
go test -v ./tests/ext-integration -run TestYahoo

# Comparison test (requires both)
go test -v ./tests/ext-integration -run TestYahooMailVsGmail
```

## Manual Verification

After running the tests, manually verify:

1. **Email Delivery**: Check that emails arrived in your inbox
2. **Email Formatting**: Verify HTML and text content render correctly
3. **Subject Lines**: Confirm subject lines are appropriate
4. **Sender Information**: Check "From" address displays correctly
5. **Spam Folder**: Ensure emails aren't marked as spam

## Test Results

The tests will output detailed logs showing:
- ✅ Success indicators for each email type sent
- Email content details (recipient, subject, key information)
- Error messages for failures
- Summary of all tests executed

## Security Notes

⚠️ **Important Security Practices:**

1. **Never commit credentials** to version control
2. **Use app passwords**, not account passwords
3. **Rotate passwords** regularly
4. **Limit app password scope** to mail only
5. **Revoke app passwords** when no longer needed

## Troubleshooting

### Gmail Issues

**"Username and Password not accepted"**
- Ensure 2-Step Verification is enabled
- Use an App Password, not your account password
- Check that the app password is entered correctly (no spaces)

**"Less secure app access"**
- Gmail no longer supports this; you must use App Passwords

### Yahoo Mail Issues

**"Authentication failed"**
- Generate a new app password from Yahoo Account Security
- Ensure you're using the app password, not your account password
- Check that your Yahoo account is in good standing

**"Connection timeout"**
- Verify SMTP server: `smtp.mail.yahoo.com`
- Verify port: `587` (TLS/STARTTLS)
- Check firewall settings

### General Issues

**"Dial tcp: i/o timeout"**
- Check internet connection
- Verify firewall isn't blocking port 587
- Try a different network

**"Certificate verification failed"**
- Ensure system time is correct
- Update system certificates

## CI/CD Integration

These tests are **not** intended for automated CI/CD pipelines because:
- They require valid external credentials
- They send real emails
- They depend on external service availability
- They may be rate-limited

For CI/CD, use the E2E tests in `tests/e2e/` which mock external dependencies.

## Adding New Email Providers

To add tests for a new email provider:

1. Create a new test file: `tests/ext-integration/provider_test.go`
2. Follow the pattern from `gmail_test.go` or `yahoo_test.go`
3. Update this README with provider-specific instructions
4. Document SMTP settings (host, port, TLS requirements)
5. Include app password generation instructions

## SMTP Configuration Reference

| Provider | SMTP Host | Port | TLS | App Password Required |
|----------|-----------|------|-----|----------------------|
| Gmail | smtp.gmail.com | 587 | STARTTLS | Yes |
| Yahoo Mail | smtp.mail.yahoo.com | 587 | STARTTLS | Yes |

## Future Enhancements

Potential additions to external integration tests:
- [ ] Outlook/Office 365 integration
- [ ] SendGrid integration
- [ ] AWS SES integration
- [ ] Email delivery tracking
- [ ] Bounce/complaint handling tests
- [ ] Rate limiting tests
