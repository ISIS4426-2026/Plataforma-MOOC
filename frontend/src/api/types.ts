// Tipos de las respuestas de la API (api/openapi.yaml). Solo lo que la interfaz usa.

export interface Course {
  id: string;
  /** Identidad estable del curso entre versiones: es la que usan inscripciones y progreso. */
  stable_id: string;
  title: string;
  description?: string;
  version: number;
  status: "draft" | "published" | "unpublished";
  author_id?: string;
}

export interface Page<T> {
  items: T[];
  pagination: { next_cursor: string | null; has_more: boolean };
}

export interface Enrollment {
  id: string;
  student_id: string;
  course_stable_id: string;
  status: "active" | "withdrawn";
  enrolled_at: string;
}

export interface Module {
  id: string;
  stable_id: string;
  course_id: string;
  title: string;
  position: number;
}

export interface Unit {
  id: string;
  stable_id: string;
  module_id: string;
  title: string;
  position: number;
}

export type ResourceType =
  | "text"
  | "image"
  | "video"
  | "audio"
  | "pdf"
  | "presentation"
  | "downloadable"
  | "iframe"
  | "external_link"
  | "quiz";

/**
 * El enunciado habla de un recurso «available»; en el codigo ese estado se llama `completed`
 * (nota 8 de NOTAS_TECNICAS). Un recurso que no esta `completed` NUNCA se muestra como disponible.
 */
export type ProcessingStatus = "pending" | "processing" | "completed" | "failed";

export interface Resource {
  id: string;
  stable_id: string;
  unit_id: string;
  title: string;
  type: ResourceType;
  position: number;
  is_visible: boolean;
  is_mandatory: boolean;
  allow_download: boolean;
  content_text?: string;
  object_key?: string;
  processing_status?: ProcessingStatus;
}

export interface Progress {
  course_stable_id: string;
  completed_resource_stable_ids: string[];
  completed_count: number;
  total_count: number;
  percentage: number;
  is_approved: boolean;
  badge?: { id: string; verification_code: string; issued_at: string; revoked: boolean };
}

export interface QuizOption {
  id: string;
  text: string;
  position: number;
}

export interface QuizQuestion {
  id: string;
  prompt: string;
  position: number;
  options: QuizOption[];
}

/** Vista del estudiante: nunca trae `is_correct`. */
export interface Quiz {
  id: string;
  resource_id: string;
  title: string;
  passing_score: number;
  max_attempts: number;
  questions: QuizQuestion[];
}

export interface QuizAnswer {
  question_id: string;
  selected_option_id: string;
}

export interface QuizAttempt {
  attempt: number;
  quiz: Quiz;
  answers: QuizAnswer[];
  started_at: string;
  updated_at: string;
  expires_at?: string;
  expired: boolean;
}

export interface QuizSubmission {
  submission_id: string;
  score: number;
  passed: boolean;
  attempt: number;
  attempts_remaining?: number;
  submitted_at?: string;
}
