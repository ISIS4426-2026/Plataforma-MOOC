-- Section 3 of the spec lists "posibilidad de descarga" as a per-resource
-- property alongside visibility and mandatoriness, independent of the
-- resource's type (a video can be streamed-only or downloadable, same as a
-- PDF). 000001_init_schema.up.sql captured is_visible and is_mandatory but
-- missed this one; issue #19 (CRUD de Modulo, Unidad y Recurso) is what
-- surfaces the gap, since it is what exposes every field the spec lists.

ALTER TABLE resources
    ADD COLUMN IF NOT EXISTS allow_download BOOLEAN NOT NULL DEFAULT TRUE;
