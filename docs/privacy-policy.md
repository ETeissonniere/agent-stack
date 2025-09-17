---
title: Privacy Policy
layout: default
---

# Privacy Policy

Effective Date: 2025-09-17

YouTube Curator ("the Service") analyzes your YouTube subscription feed and sends curated email digests powered by AI. This policy explains what data the Service collects, how it is used, and the choices available to you.

## 1. Data We Collect
- **Tokens:** Google OAuth refresh/access tokens to authenticate with the YouTube Data API.
- **Video metadata:** Titles, channel names, durations, publish dates, thumbnails, and video IDs retrieved from the YouTube Data API.
- **AI annotations:** Relevance scores and short summaries generated from video metadata using Gemini 2.5 Flash.
- **Email diagnostics:** Recipient address, timestamp, delivery status, and basic run metrics for health monitoring.

## 2. How We Use Data
- Authenticate with YouTube to fetch videos from your subscriptions.
- Analyze metadata with Gemini to rank relevance and build the digest email.
- Send HTML email reports via your configured SMTP account.
- Track scheduler health (runtime, success, recoverable errors) to surface issues.

## 3. Data Storage & Retention
- OAuth tokens are stored encrypted on disk at the configured `youtube.token_file` solely to refresh access when needed.
- Video metadata and AI annotations stay in memory during processing and are discarded after emails are sent.
- Email delivery logs live locally for up to 30 days for troubleshooting, then auto-delete.

## 4. Sharing & Disclosure
- Data is not sold or shared with third parties. Google APIs and Gemini are accessed directly with your credentials; no intermediaries receive your data.
- Information may be disclosed if required by law or to protect the Service’s integrity.

## 5. Security
- Tokens and configuration secrets are stored on the host machine with restrictive filesystem permissions.
- OAuth exchanges and SMTP sessions use TLS/HTTPS.

## 6. Your Choices
- Update or remove credentials anytime by editing `config.yaml` or running the setup flow again.
- Delete the token file and configuration to revoke access. You can also revoke OAuth permissions from your Google Account security page.

## 7. Children’s Privacy
The Service is intended for adult users managing their own subscriptions and is not directed to children under 16.

## 8. Changes
Policy updates will be published here with a revised effective date. Continued use after updates means you accept the new policy.

## 9. Contact
Questions or requests? Open an issue on the project's GitHub repository.
