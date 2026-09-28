-- ============================================================================
-- Verificacion independiente del estado tras una corrida del escenario 1
-- (issue #132 / H2). Lee directo de la base, sin pasar por la API: es la
-- comprobacion de que el guion no solo vio "200", sino que el estado quedo bien.
--
-- Con N usuarios virtuales y 3 sesiones cada uno se espera:
--   estudiantes_con_envios = N,  min = max = 3 envios,  envios_totales = 3 x N
--   inscripciones_activas  = N
--   envios_duplicados_mismo_intento = 0   (los reenvios con la misma
--                                          Idempotency-Key no calificaron dos veces)
-- Solo lee. No cambia nada.
-- ============================================================================

SELECT count(*) AS estudiantes_con_envios, min(n) AS min_envios, max(n) AS max_envios, sum(n) AS envios_totales
  FROM (SELECT student_id, count(*) AS n FROM quiz_submissions
         WHERE student_id::text LIKE 'c9%' GROUP BY student_id) t;

SELECT count(*) AS inscripciones_activas
  FROM enrollments WHERE student_id::text LIKE 'c9%' AND status = 'active';

SELECT count(*) AS insignias_emitidas
  FROM badges WHERE student_id::text LIKE 'c9%';

SELECT count(*) AS envios_duplicados_mismo_intento
  FROM (SELECT student_id, attempt FROM quiz_submissions
         WHERE student_id::text LIKE 'c9%' GROUP BY student_id, attempt HAVING count(*) > 1) d;
