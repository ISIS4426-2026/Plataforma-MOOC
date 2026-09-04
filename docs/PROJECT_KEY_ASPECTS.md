# Plataforma MOOC — Key Technical & Functional Aspects for AI Agents

> [!IMPORTANT]
> This document summarizes the critical architectural rules, domain specifications, functional requirements, and security guidelines extracted from the official project specification (`docs/2026-20 proyecto-plataforma-mooc (2).pdf`). All AI agents and developers working on this project MUST strictly follow these directives.

---

## 1. Executive Summary & Project Purpose

* **Project Goal**: Build a web-based Massive Open Online Course (MOOC) platform operated by a single organization to maintain full control over content, student data, brand identity, and product evolution.
* **Scale Target**: Initial capacity of up to **50,000 registered users** and **2,000 concurrent users**.
* **Development Horizon**: MVP delivery across **4 to 5 releases over 13 weeks** by a 4-person team.
* **AI Usage**: Total usage of AI tools is permitted for product development.
* **Core Technology Stack**:
  * **Backend**: Go (Modular Monolith architecture with independent background workers).
  * **Frontend**: Student choice of modern framework, communicating strictly via REST JSON over HTTPS.
  * **Database**: PostgreSQL (Transactional source of truth).
  * **In-Memory Store & Task Queue**: Redis (Sessions, cache, rate limiting, and [Asynq](https://github.com/hibiken/asynq) queue).
  * **Object Storage**: AWS S3 / MinIO (Binary assets, HLS segments, PDFs, badges).
  * **Local Environment**: Docker Compose including **MinIO** and **Mailpit** (email testing).

---

## 2. Core Roles & Authorization Model

The system defines **three global roles**:

1. **Administrator (`Administrador`)**:
   * Manages user accounts, roles, statuses, active sessions, and immutable audit logs.
   * **Rule**: Instructor (`Profesor`) accounts can ONLY be created by an Administrator. Public self-registration for instructors is prohibited.
   * **Rule**: The system must protect the last active administrator account from being deleted or revoked.
2. **Instructor (`Profesor`)**:
   * Creates, structures, and manages course content (`Curso` → `Módulo` → `Unidad` → `Recurso`).
   * Previews course drafts, manages versioning, and requests publication.
3. **Student (`Estudiante`)**:
   * Public registration with mandatory email verification, session recovery, and revocable sessions.
   * Browses course catalog, enrolls/withdraws from courses, consumes content asynchronously (HLS streaming, PDF viewer), completes quizzes, tracks validated progress, and earns verifiable digital badges.

---

## 3. Academic Structure & Content Management

### Hierarchy
```
Curso (Root Aggregate & Version Container)
 └── Módulo (Ordered)
      └── Unidad (Ordered)
           └── Recurso (Visible/Hidden, Mandatory/Optional)
```

### Supported Resource Types
* Enriched Text (persisted strictly as **Canonical Extended Markdown**)
* Image, Video, Audio, PDF, Presentation, Downloadable File
* Authorized Iframe (whitelisted sandbox)
* External Link
* Multiple-Choice Quiz

### Versioning & Immutability Rules
* **Published Version Inmutability**: Once a course version is published, it becomes **strictly immutable**.
* **Update Drafts**: Any subsequent edits must occur on an update draft (`borrador de actualización`). In the MVP stage, editing a published course requires temporarily unpublishing it.
* **Stable Identifiers (`stable_id`)**: Content items use stable identifiers across versions. When a new course version is published, student progress is preserved via these stable IDs.
* **Binary Asset Storage**: Binaries (images, videos, audio, PDFs, badges) MUST NEVER be stored in the relational database. They reside exclusively in Object Storage (S3/MinIO) and are served directly or via CDN using **presigned URLs**.

---

## 4. Required Architecture & Infrastructure Rules

### Modular Monolith in Go
* **Decoupled Architecture**: Domain logic must be cleanly decoupled from the HTTP web framework and cloud infrastructure providers (Hexagonal / Clean Architecture).
* **Stateless API & Workers**: API nodes and Go background workers must be completely stateless to allow seamless horizontal scaling.
* **API Specifications**:
  * Base URL path: `/api/v1`
  * Specification standard: **OpenAPI 3.1**
  * Mandatory headers & standards: Uniform error responses, cursor-based pagination, `ETag` caching, and `Idempotency-Key` headers for write operations.

### Async Background Processing (Workers)
* Handled via **Redis + Asynq**.
* Tasks include: Transcoding video/audio to **HLS** (without upscaling), PDF conversion, malware scanning, MIME verification.
* **Idempotency & Retries**: Worker tasks must be strictly idempotent. Failed jobs retry with exponential backoff up to 3 times before moving to a **Dead-Letter Queue (DLQ)** and raising an alert.
* **Data Safety**: PostgreSQL remains the transactional source of truth. Un-heartbeated/stuck jobs must be safely re-enqueued to prevent loss due to Redis memory eviction.

### Resilience Targets
* **RPO (Recovery Point Objective)** $\le$ 15 minutes.
* **RTO (Recovery Time Objective)** $\le$ 4 hours.

---

## 5. Critical Non-Negotiable Guardrails for Agents

> [!CAUTION]
> AI agents MUST NOT violate the following strict domain and security constraints:

1. **Quiz Key Secrecy**: The correct answer key for a quiz MUST NEVER be sent to the client/frontend. Quiz evaluation and scoring must happen strictly server-side.
2. **Server-Validated Progress**: Client-submitted progress percentages MUST be rejected and audited as potential manipulation. Student progress is determined exclusively by server-verified heartbeats, minimum dwell time, and resource opening events.
3. **Badge Integrity & Privacy**:
   * Badges are emitted idempotently when a student reaches the `approved` state.
   * Emitted badges provide a unique public verification URL.
   * **Privacy**: The public badge verification URL MUST NOT expose the student's email address.
4. **Presigned Direct Uploads**: Binary files are uploaded directly from the client to object storage via 24-hour resumable presigned URLs. The API server must validate integrity (checksum), real MIME type, and malware status without proxying heavy binary streams through the API main loop.
5. **No Binaries in PostgreSQL**: Relational tables store only metadata, file keys, URLs, and states.

---

## 6. Functional Scope Overview

### Mandatory Minimum Scope (MVP - Must-Have)
1. **Identity & Access**: Student self-registration with email verification; Admin-only professor creation; Protected last active admin; Session revocation.
2. **User & Audit Management**: Role assignment, status control, session management, and immutable audit logs.
3. **Course Authoring**: 4-level hierarchy, ordering, draft previsualization, strict publication validation, immutable published versions.
4. **Block Editor**: Extended Markdown editor with autosave and recovery features.
5. **Direct Multipart Upload**: 24h resumable uploads directly to S3/MinIO, checksum verification, MIME check, anti-malware scan.
6. **Async Media Processing**: Asynchronous Go workers for HLS audio/video encoding (preserving originals), CDN delivery, idempotency.
7. **Accessible Consumption**: PDF viewer, adaptive video/audio player with resume-from-last-position support.
8. **Academic Quizzes**: Multiple-choice, attempt limits, partial save, server-side grading, configurable feedback policies.
9. **Validated Progress & Badging**: Mandatory resource checks, server-evaluated progress, idempotent badge emission with public verification URL.
10. **Catalog & Enrollment**: Course search/filters, enrollment/withdrawal/re-enrollment while preserving historical progress.

### Optional Scope (Should / Evaluation Bonus)
* Presentation conversion (PPTX/ODP to PDF) with preview.
* Whitelisted/sandboxed HTML iframe embed policy.
* Full-featured rich text editor (tables, formulas, revision history).
* Update drafts with change classification and automated progress migration.
* Advanced Admin Dashboard with aggregated quiz analytics.
* Co-authoring, personal data export, internationalization (i18n), Open Badges 3.0, and async course forums.

---

## 7. Quality Assurance & Verification Requirements

Acceptance must be demonstrated via reproducible automated tests and telemetry:
* **E2E Testing**: All 9 critical flows (Identity, Authoring, Upload, Media Processing, Consumption, Quiz, Progress/Badges, Catalog, Load/Recovery) must pass E2E tests.
* **Load Testing**: Demonstrate target concurrency (2,000 concurrent users) and p95 latency thresholds.
* **Accessibility**: Must meet **WCAG 2.2 AA** standards (verified via automated accessibility audits).
* **Observability**: Full integration with **OpenTelemetry** for logs, metrics, correlated traces, and alert rules.
* **CI/CD Pipeline**: Automated CI pipeline verifying `build`, `lint`, `security scan`, `database migrations`, and `tests` prior to deployment.

---

## 8. Checklist for Future AI Agents Working on this Repo

Before submitting any code changes, ensure:
- [ ] Go domain models are kept independent of HTTP routing and database drivers.
- [ ] No database schema stores binary BLOBs.
- [ ] REST API endpoints follow `/api/v1/...` and include OpenAPI 3.1 documentation.
- [ ] All new write endpoints accept an `Idempotency-Key`.
- [ ] Quiz answers are never leaked in API JSON responses.
- [ ] Progress logic relies strictly on server-computed events/heartbeats.
- [ ] Background worker tasks implement Asynq handlers with DLQ support.
- [ ] All Docker Compose services (Go API, Workers, Postgres, Redis, MinIO, Mailpit) start up cleanly.
- [ ] Automated tests (unit, integration, E2E) are included and passing.
