package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// This file backs issue #19: Module, Unit and Resource CRUD respecting the
// Curso -> Módulo -> Unidad -> Recurso hierarchy. See the doc comment on
// ModuleRepository in internal/domain/course.go for the conventions shared
// by all three (client-generated IDs, audit entry in the same transaction,
// ErrCourseImmutable once the course is published).
//
// Every write locks its parent chain up to and including the courses row
// with `FOR UPDATE OF`, in one query. That serves two purposes at once: it
// is what makes "the parent must exist" (ErrNotFound otherwise) and "the
// course must not be published" (ErrCourseImmutable otherwise) atomic with
// the write that follows, and it is what serializes two concurrent inserts
// under the same parent so the sibling count read to assign a position is
// never stale by the time the row lands.

const moduleColumns = `id, stable_id, course_id, title, position, created_at`
const unitColumns = `id, stable_id, module_id, title, position, created_at`
const resourceColumns = `id, stable_id, unit_id, title, type, position, is_visible, is_mandatory, allow_download, content_text, object_key, processing_status, created_at`

// ---- ModuleRepository ----------------------------------------------------

type ModuleRepository struct{ db *sql.DB }

var _ domain.ModuleRepository = (*ModuleRepository)(nil)

func NewModuleRepository(db *sql.DB) *ModuleRepository { return &ModuleRepository{db: db} }

func (r *ModuleRepository) Create(ctx context.Context, module *domain.Module, entry *domain.AuditEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status domain.CourseStatus
	err = tx.QueryRowContext(ctx,
		`SELECT c.status FROM courses c WHERE c.id = $1 FOR UPDATE`,
		module.CourseID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("course %s: %w", module.CourseID, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("lock course %s: %w", module.CourseID, err)
	}
	if status == domain.CourseStatusPublished {
		return fmt.Errorf("course %s: %w", module.CourseID, domain.ErrCourseImmutable)
	}

	var siblingCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM modules WHERE course_id = $1`, module.CourseID).Scan(&siblingCount); err != nil {
		return fmt.Errorf("count sibling modules: %w", err)
	}
	module.Position = siblingCount

	const query = `
		INSERT INTO modules (id, stable_id, course_id, title, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`
	if err := tx.QueryRowContext(ctx, query,
		module.ID, module.StableID, module.CourseID, module.Title, module.Position,
	).Scan(&module.CreatedAt); err != nil {
		return fmt.Errorf("create module: %w", err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit module creation: %w", err)
	}
	return nil
}

func (r *ModuleRepository) GetByID(ctx context.Context, id string) (*domain.Module, error) {
	query := `SELECT ` + moduleColumns + ` FROM modules WHERE id = $1`
	module, err := scanModule(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get module %s: %w", id, err)
	}
	return module, nil
}

func (r *ModuleRepository) ListByCourse(ctx context.Context, courseID string) ([]*domain.Module, error) {
	query := `SELECT ` + moduleColumns + ` FROM modules WHERE course_id = $1 ORDER BY position ASC`
	rows, err := r.db.QueryContext(ctx, query, courseID)
	if err != nil {
		return nil, fmt.Errorf("list modules of course %s: %w", courseID, err)
	}
	defer rows.Close()

	var modules []*domain.Module
	for rows.Next() {
		module, err := scanModule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan module: %w", err)
		}
		modules = append(modules, module)
	}
	return modules, rows.Err()
}

func (r *ModuleRepository) Update(ctx context.Context, moduleID, title string, entry *domain.AuditEntry) (*domain.Module, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockModuleAndCourse(ctx, tx, moduleID); err != nil {
		return nil, err
	}

	const query = `UPDATE modules SET title = $2 WHERE id = $1 RETURNING ` + moduleColumns
	updated, err := scanModule(tx.QueryRowContext(ctx, query, moduleID, title))
	if err != nil {
		return nil, fmt.Errorf("update module %s: %w", moduleID, err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit module update: %w", err)
	}
	return updated, nil
}

func (r *ModuleRepository) Delete(ctx context.Context, moduleID string, entry *domain.AuditEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockModuleAndCourse(ctx, tx, moduleID); err != nil {
		return err
	}

	var courseID string
	var position int
	if err := tx.QueryRowContext(ctx, `SELECT course_id, position FROM modules WHERE id = $1`, moduleID).
		Scan(&courseID, &position); err != nil {
		return fmt.Errorf("read module %s before delete: %w", moduleID, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM modules WHERE id = $1`, moduleID); err != nil {
		return fmt.Errorf("delete module %s: %w", moduleID, err)
	}

	// Close the gap the deleted module leaves, so positions stay a dense
	// 0..n-1 sequence (internal/domain/ordering.go).
	if _, err := tx.ExecContext(ctx,
		`UPDATE modules SET position = position - 1 WHERE course_id = $1 AND position > $2`,
		courseID, position,
	); err != nil {
		return fmt.Errorf("renumber sibling modules after delete: %w", err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit module deletion: %w", err)
	}
	return nil
}

// lockModuleAndCourse locks a module and its owning course in one round
// trip, and enforces both preconditions every module write shares: the
// module must exist, and its course must not be published.
func lockModuleAndCourse(ctx context.Context, tx *sql.Tx, moduleID string) error {
	var status domain.CourseStatus
	err := tx.QueryRowContext(ctx,
		`SELECT c.status FROM modules m JOIN courses c ON c.id = m.course_id WHERE m.id = $1 FOR UPDATE OF m, c`,
		moduleID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("module %s: %w", moduleID, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("lock module %s: %w", moduleID, err)
	}
	if status == domain.CourseStatusPublished {
		return fmt.Errorf("module %s: %w", moduleID, domain.ErrCourseImmutable)
	}
	return nil
}

func scanModule(scanner rowScanner) (*domain.Module, error) {
	var m domain.Module
	err := scanner.Scan(&m.ID, &m.StableID, &m.CourseID, &m.Title, &m.Position, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ---- UnitRepository -------------------------------------------------------

type UnitRepository struct{ db *sql.DB }

var _ domain.UnitRepository = (*UnitRepository)(nil)

func NewUnitRepository(db *sql.DB) *UnitRepository { return &UnitRepository{db: db} }

func (r *UnitRepository) Create(ctx context.Context, unit *domain.Unit, entry *domain.AuditEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status domain.CourseStatus
	err = tx.QueryRowContext(ctx,
		`SELECT c.status FROM modules m JOIN courses c ON c.id = m.course_id WHERE m.id = $1 FOR UPDATE OF m, c`,
		unit.ModuleID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("module %s: %w", unit.ModuleID, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("lock module %s: %w", unit.ModuleID, err)
	}
	if status == domain.CourseStatusPublished {
		return fmt.Errorf("module %s: %w", unit.ModuleID, domain.ErrCourseImmutable)
	}

	var siblingCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM units WHERE module_id = $1`, unit.ModuleID).Scan(&siblingCount); err != nil {
		return fmt.Errorf("count sibling units: %w", err)
	}
	unit.Position = siblingCount

	const query = `
		INSERT INTO units (id, stable_id, module_id, title, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`
	if err := tx.QueryRowContext(ctx, query,
		unit.ID, unit.StableID, unit.ModuleID, unit.Title, unit.Position,
	).Scan(&unit.CreatedAt); err != nil {
		return fmt.Errorf("create unit: %w", err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit unit creation: %w", err)
	}
	return nil
}

func (r *UnitRepository) GetByID(ctx context.Context, id string) (*domain.Unit, error) {
	query := `SELECT ` + unitColumns + ` FROM units WHERE id = $1`
	unit, err := scanUnit(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get unit %s: %w", id, err)
	}
	return unit, nil
}

func (r *UnitRepository) ListByModule(ctx context.Context, moduleID string) ([]*domain.Unit, error) {
	query := `SELECT ` + unitColumns + ` FROM units WHERE module_id = $1 ORDER BY position ASC`
	rows, err := r.db.QueryContext(ctx, query, moduleID)
	if err != nil {
		return nil, fmt.Errorf("list units of module %s: %w", moduleID, err)
	}
	defer rows.Close()

	var units []*domain.Unit
	for rows.Next() {
		unit, err := scanUnit(rows)
		if err != nil {
			return nil, fmt.Errorf("scan unit: %w", err)
		}
		units = append(units, unit)
	}
	return units, rows.Err()
}

func (r *UnitRepository) Update(ctx context.Context, unitID, title string, entry *domain.AuditEntry) (*domain.Unit, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockUnitAndCourse(ctx, tx, unitID); err != nil {
		return nil, err
	}

	const query = `UPDATE units SET title = $2 WHERE id = $1 RETURNING ` + unitColumns
	updated, err := scanUnit(tx.QueryRowContext(ctx, query, unitID, title))
	if err != nil {
		return nil, fmt.Errorf("update unit %s: %w", unitID, err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit unit update: %w", err)
	}
	return updated, nil
}

func (r *UnitRepository) Delete(ctx context.Context, unitID string, entry *domain.AuditEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockUnitAndCourse(ctx, tx, unitID); err != nil {
		return err
	}

	var moduleID string
	var position int
	if err := tx.QueryRowContext(ctx, `SELECT module_id, position FROM units WHERE id = $1`, unitID).
		Scan(&moduleID, &position); err != nil {
		return fmt.Errorf("read unit %s before delete: %w", unitID, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM units WHERE id = $1`, unitID); err != nil {
		return fmt.Errorf("delete unit %s: %w", unitID, err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE units SET position = position - 1 WHERE module_id = $1 AND position > $2`,
		moduleID, position,
	); err != nil {
		return fmt.Errorf("renumber sibling units after delete: %w", err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit unit deletion: %w", err)
	}
	return nil
}

func lockUnitAndCourse(ctx context.Context, tx *sql.Tx, unitID string) error {
	var status domain.CourseStatus
	err := tx.QueryRowContext(ctx,
		`SELECT c.status FROM units u
			JOIN modules m ON m.id = u.module_id
			JOIN courses c ON c.id = m.course_id
			WHERE u.id = $1 FOR UPDATE OF u, c`,
		unitID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("unit %s: %w", unitID, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("lock unit %s: %w", unitID, err)
	}
	if status == domain.CourseStatusPublished {
		return fmt.Errorf("unit %s: %w", unitID, domain.ErrCourseImmutable)
	}
	return nil
}

func scanUnit(scanner rowScanner) (*domain.Unit, error) {
	var u domain.Unit
	err := scanner.Scan(&u.ID, &u.StableID, &u.ModuleID, &u.Title, &u.Position, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ---- ResourceRepository ----------------------------------------------------

type ResourceRepository struct{ db *sql.DB }

var _ domain.ResourceRepository = (*ResourceRepository)(nil)

func NewResourceRepository(db *sql.DB) *ResourceRepository { return &ResourceRepository{db: db} }

func (r *ResourceRepository) Create(ctx context.Context, resource *domain.Resource, entry *domain.AuditEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status domain.CourseStatus
	err = tx.QueryRowContext(ctx,
		`SELECT c.status FROM units u
			JOIN modules m ON m.id = u.module_id
			JOIN courses c ON c.id = m.course_id
			WHERE u.id = $1 FOR UPDATE OF u, c`,
		resource.UnitID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("unit %s: %w", resource.UnitID, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("lock unit %s: %w", resource.UnitID, err)
	}
	if status == domain.CourseStatusPublished {
		return fmt.Errorf("unit %s: %w", resource.UnitID, domain.ErrCourseImmutable)
	}

	var siblingCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE unit_id = $1`, resource.UnitID).Scan(&siblingCount); err != nil {
		return fmt.Errorf("count sibling resources: %w", err)
	}
	resource.Position = siblingCount

	const query = `
		INSERT INTO resources (id, stable_id, unit_id, title, type, position, is_visible, is_mandatory, allow_download, content_text, object_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING processing_status, created_at`
	if err := tx.QueryRowContext(ctx, query,
		resource.ID, resource.StableID, resource.UnitID, resource.Title, string(resource.Type), resource.Position,
		resource.IsVisible, resource.IsMandatory, resource.AllowDownload,
		nullString(resource.ContentText), nullString(resource.ObjectKey),
	).Scan(&resource.ProcessingStatus, &resource.CreatedAt); err != nil {
		return fmt.Errorf("create resource: %w", err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit resource creation: %w", err)
	}
	return nil
}

func (r *ResourceRepository) GetByID(ctx context.Context, id string) (*domain.Resource, error) {
	query := `SELECT ` + resourceColumns + ` FROM resources WHERE id = $1`
	resource, err := scanResource(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get resource %s: %w", id, err)
	}
	return resource, nil
}

func (r *ResourceRepository) ListByUnit(ctx context.Context, unitID string) ([]*domain.Resource, error) {
	query := `SELECT ` + resourceColumns + ` FROM resources WHERE unit_id = $1 ORDER BY position ASC`
	rows, err := r.db.QueryContext(ctx, query, unitID)
	if err != nil {
		return nil, fmt.Errorf("list resources of unit %s: %w", unitID, err)
	}
	defer rows.Close()

	var resources []*domain.Resource
	for rows.Next() {
		resource, err := scanResource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan resource: %w", err)
		}
		resources = append(resources, resource)
	}
	return resources, rows.Err()
}

func (r *ResourceRepository) Update(ctx context.Context, resourceID string, fields domain.ResourceUpdate, entry *domain.AuditEntry) (*domain.Resource, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockResourceAndCourse(ctx, tx, resourceID); err != nil {
		return nil, err
	}

	const query = `
		UPDATE resources
		SET title = $2, is_visible = $3, is_mandatory = $4, allow_download = $5
		WHERE id = $1
		RETURNING ` + resourceColumns
	updated, err := scanResource(tx.QueryRowContext(ctx, query,
		resourceID, fields.Title, fields.IsVisible, fields.IsMandatory, fields.AllowDownload))
	if err != nil {
		return nil, fmt.Errorf("update resource %s: %w", resourceID, err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit resource update: %w", err)
	}
	return updated, nil
}

func (r *ResourceRepository) Delete(ctx context.Context, resourceID string, entry *domain.AuditEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockResourceAndCourse(ctx, tx, resourceID); err != nil {
		return err
	}

	var unitID string
	var position int
	if err := tx.QueryRowContext(ctx, `SELECT unit_id, position FROM resources WHERE id = $1`, resourceID).
		Scan(&unitID, &position); err != nil {
		return fmt.Errorf("read resource %s before delete: %w", resourceID, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM resources WHERE id = $1`, resourceID); err != nil {
		return fmt.Errorf("delete resource %s: %w", resourceID, err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE resources SET position = position - 1 WHERE unit_id = $1 AND position > $2`,
		unitID, position,
	); err != nil {
		return fmt.Errorf("renumber sibling resources after delete: %w", err)
	}

	if err := insertAuditEntry(ctx, tx, entry); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit resource deletion: %w", err)
	}
	return nil
}

func lockResourceAndCourse(ctx context.Context, tx *sql.Tx, resourceID string) error {
	var status domain.CourseStatus
	err := tx.QueryRowContext(ctx,
		`SELECT c.status FROM resources r
			JOIN units u ON u.id = r.unit_id
			JOIN modules m ON m.id = u.module_id
			JOIN courses c ON c.id = m.course_id
			WHERE r.id = $1 FOR UPDATE OF r, c`,
		resourceID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("resource %s: %w", resourceID, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("lock resource %s: %w", resourceID, err)
	}
	if status == domain.CourseStatusPublished {
		return fmt.Errorf("resource %s: %w", resourceID, domain.ErrCourseImmutable)
	}
	return nil
}

func scanResource(scanner rowScanner) (*domain.Resource, error) {
	var (
		res              domain.Resource
		resourceType     string
		contentText      sql.NullString
		objectKey        sql.NullString
		processingStatus sql.NullString
	)

	err := scanner.Scan(
		&res.ID, &res.StableID, &res.UnitID, &res.Title, &resourceType, &res.Position,
		&res.IsVisible, &res.IsMandatory, &res.AllowDownload,
		&contentText, &objectKey, &processingStatus, &res.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	res.Type = domain.ResourceType(resourceType)
	res.ContentText = contentText.String
	res.ObjectKey = objectKey.String
	res.ProcessingStatus = processingStatus.String

	return &res, nil
}
