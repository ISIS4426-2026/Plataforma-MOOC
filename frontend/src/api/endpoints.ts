import { newIdempotencyKey, request } from "./client";
import type {
  Course,
  Enrollment,
  Module,
  Page,
  Progress,
  Quiz,
  QuizAnswer,
  QuizAttempt,
  QuizSubmission,
  Resource,
  Unit,
} from "./types";

const data = <T>(p: Promise<{ data: T }>): Promise<T> => p.then((r) => r.data);

// ---- Catalogo ------------------------------------------------------------------------

export function listCourses(params: { search?: string; cursor?: string; limit?: number } = {}): Promise<Page<Course>> {
  const q = new URLSearchParams();
  if (params.search) q.set("search", params.search);
  if (params.cursor) q.set("cursor", params.cursor);
  q.set("limit", String(params.limit ?? 50));
  return data(request<Page<Course>>(`/courses?${q}`));
}

/**
 * Las inscripciones y el progreso hablan de `course_stable_id`, pero `GET /courses/{id}` solo
 * acepta el id de la version. Para poner titulo a «mis cursos» hay que cruzar con el catalogo
 * (nota 27 de NOTAS_TECNICAS: hueco del contrato). Se recorre una vez y se guarda en memoria.
 */
let indexCache: Promise<Course[]> | null = null;

export function courseIndex(force = false): Promise<Course[]> {
  if (!indexCache || force) {
    indexCache = (async () => {
      const all: Course[] = [];
      let cursor: string | undefined;
      do {
        const page = await listCourses({ cursor, limit: 100 });
        all.push(...page.items);
        cursor = page.pagination.has_more ? (page.pagination.next_cursor ?? undefined) : undefined;
      } while (cursor);
      return all;
    })().catch((e) => {
      indexCache = null;
      throw e;
    });
  }
  return indexCache;
}

export const getCourse = (id: string): Promise<Course> => data(request<Course>(`/courses/${id}`));

// ---- Inscripcion ---------------------------------------------------------------------

export const myEnrollments = (): Promise<Enrollment[]> =>
  data(request<{ items: Enrollment[] }>("/me/enrollments")).then((d) => d.items);

export const enroll = (courseId: string, key = newIdempotencyKey()): Promise<Enrollment> =>
  data(request<Enrollment>(`/courses/${courseId}/enrollments`, { method: "POST", body: {}, idempotencyKey: key }));

// ---- Estructura del curso ------------------------------------------------------------

export const listModules = (courseId: string): Promise<Module[]> =>
  data(request<{ items: Module[] }>(`/courses/${courseId}/modules`)).then((d) => d.items);

export const listUnits = (moduleId: string): Promise<Unit[]> =>
  data(request<{ items: Unit[] }>(`/modules/${moduleId}/units`)).then((d) => d.items);

export const listResources = (unitId: string): Promise<Resource[]> =>
  data(request<{ items: Resource[] }>(`/units/${unitId}/resources`)).then((d) => d.items);

export interface CourseTree {
  modules: Array<Module & { units: Array<Unit & { resources: Resource[] }> }>;
}

/** Modulos → unidades → recursos, pedidos en paralelo por nivel. */
export async function loadCourseTree(courseId: string): Promise<CourseTree> {
  const modules = (await listModules(courseId)).sort((a, b) => a.position - b.position);
  const withUnits = await Promise.all(
    modules.map(async (m) => {
      const units = (await listUnits(m.id)).sort((a, b) => a.position - b.position);
      const withResources = await Promise.all(
        units.map(async (u) => ({
          ...u,
          resources: (await listResources(u.id)).filter((r) => r.is_visible).sort((a, b) => a.position - b.position),
        })),
      );
      return { ...m, units: withResources };
    }),
  );
  return { modules: withUnits };
}

// ---- Progreso ------------------------------------------------------------------------

export const courseProgress = (courseId: string): Promise<Progress> =>
  data(request<Progress>(`/progress/courses/${courseId}`));

/**
 * Un latido recalcula el progreso en el servidor; repetirlo es inofensivo (no lleva Idempotency-Key
 * a proposito). La interfaz solo muestra lo que el servidor responde.
 */
export const heartbeat = (resourceId: string, dwellTimeSeconds: number, completed: boolean): Promise<Progress> =>
  data(
    request<Progress>("/progress/heartbeat", {
      method: "POST",
      body: { resource_id: resourceId, dwell_time_seconds: dwellTimeSeconds, completed },
    }),
  );

// ---- Archivos ------------------------------------------------------------------------

export const downloadUrl = (resourceId: string): Promise<{ download_url: string; expires_at: string }> =>
  data(request(`/media/resources/${resourceId}/download-url`));

// ---- Quiz ----------------------------------------------------------------------------

export const getQuiz = (resourceId: string): Promise<Quiz> => data(request<Quiz>(`/resources/${resourceId}/quiz`));

/** Abrir el intento dos veces devuelve el mismo (nota 17): es seguro llamarlo al entrar a la vista. */
export const openAttempt = (quizId: string): Promise<QuizAttempt> =>
  data(request<QuizAttempt>(`/quizzes/${quizId}/attempts`, { method: "POST", body: {} }));

export const saveAttempt = (quizId: string, answers: QuizAnswer[]): Promise<unknown> =>
  data(request(`/quizzes/${quizId}/attempts/current`, { method: "PUT", body: { answers } }));

/** La Idempotency-Key la fija quien llama: reenviar con la misma no califica dos veces. */
export const submitQuiz = (quizId: string, answers: QuizAnswer[], key: string): Promise<QuizSubmission> =>
  data(request<QuizSubmission>(`/quizzes/${quizId}/submissions`, { method: "POST", body: { answers }, idempotencyKey: key }));

export const mySubmissions = (quizId: string): Promise<QuizSubmission[]> =>
  data(request<{ items: QuizSubmission[] }>(`/quizzes/${quizId}/submissions/me`)).then((d) => d.items);
