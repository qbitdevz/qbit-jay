//go:build eval

package eval

// pair is one concept in both languages. Everything the generator writes is
// built from these tables, so no real user data can enter the corpus.
type pair struct{ en, es string }

var projects = []struct {
	name string
	// esShare is the probability that a memory of the project is in Spanish.
	esShare float64
	weight  int
}{
	{"web-shop", 0.7, 30},
	{"mobile-app", 0.2, 20},
	{"data-platform", 0.5, 20},
	{"internal-tools", 0.8, 15},
	{"billing-api", 0.3, 10},
	{"docs-site", 0.5, 5},
}

var memoryTypes = []struct {
	name   string
	weight int
}{
	{"bugfix", 20}, {"decision", 15}, {"architecture", 10}, {"discovery", 15},
	{"pattern", 10}, {"config", 10}, {"preference", 5},
}

var components = []pair{
	{"payment webhook", "webhook de pagos"},
	{"login form", "formulario de inicio de sesión"},
	{"session cache", "caché de sesiones"},
	{"invoice export", "exportación de facturas"},
	{"search index", "índice de búsqueda"},
	{"email queue", "cola de correos"},
	{"file uploader", "cargador de archivos"},
	{"user profile page", "página de perfil de usuario"},
	{"audit log", "registro de auditoría"},
	{"rate limiter", "limitador de peticiones"},
	{"notification service", "servicio de notificaciones"},
	{"report scheduler", "planificador de informes"},
	{"shopping cart", "carrito de compras"},
	{"inventory sync", "sincronización de inventario"},
	{"mobile push gateway", "pasarela de notificaciones push"},
	{"admin dashboard", "panel de administración"},
	{"permissions matrix", "matriz de permisos"},
	{"billing cycle", "ciclo de facturación"},
	{"image thumbnailer", "generador de miniaturas"},
	{"chat widget", "widget de chat"},
	{"onboarding wizard", "asistente de bienvenida"},
	{"data import job", "tarea de importación de datos"},
	{"api gateway", "puerta de enlace de la API"},
	{"feature flags", "banderas de funcionalidad"},
	{"translation catalog", "catálogo de traducciones"},
	{"geolocation lookup", "búsqueda de geolocalización"},
	{"pdf renderer", "generador de PDF"},
	{"oauth callback", "retorno de OAuth"},
	{"backup routine", "rutina de respaldo"},
	{"metrics exporter", "exportador de métricas"},
	{"order tracking", "seguimiento de pedidos"},
	{"coupon engine", "motor de cupones"},
	{"calendar sync", "sincronización de calendario"},
	{"health endpoint", "endpoint de salud"},
	{"cron runner", "ejecutor de tareas programadas"},
	{"websocket channel", "canal de websocket"},
	{"password reset", "restablecimiento de contraseña"},
	{"tenant router", "enrutador de inquilinos"},
	{"content moderation", "moderación de contenido"},
	{"analytics pipeline", "canalización de analítica"},
}

var topics = []pair{
	{"retry with exponential backoff", "reintentos con espera exponencial"},
	{"duplicate events after a timeout", "eventos duplicados tras un timeout"},
	{"memory leak in a long running worker", "fuga de memoria en un proceso de larga duración"},
	{"slow query without an index", "consulta lenta sin índice"},
	{"race condition on concurrent writes", "condición de carrera en escrituras concurrentes"},
	{"stale data after a deploy", "datos obsoletos tras el despliegue"},
	{"timezone offset error", "error de desfase de zona horaria"},
	{"unicode normalization mismatch", "diferencia de normalización unicode"},
	{"token expiry handling", "manejo de expiración de tokens"},
	{"idempotency key", "clave de idempotencia"},
	{"pagination cursor", "cursor de paginación"},
	{"optimistic locking", "bloqueo optimista"},
	{"graceful shutdown", "apagado ordenado"},
	{"circuit breaker", "cortacircuitos"},
	{"connection pool exhaustion", "agotamiento del pool de conexiones"},
	{"cold start latency", "latencia de arranque en frío"},
	{"schema migration rollback", "reversión de migración de esquema"},
	{"cache invalidation", "invalidación de caché"},
	{"N+1 query problem", "problema de consultas N+1"},
	{"flaky integration test", "test de integración inestable"},
	{"CORS preflight failure", "fallo de preflight CORS"},
	{"large payload rejection", "rechazo de cargas muy grandes"},
	{"dead letter handling", "gestión de mensajes fallidos"},
	{"locale fallback", "idioma alternativo por defecto"},
	{"rounding of currency amounts", "redondeo de importes monetarios"},
	{"webhook signature verification", "verificación de firma del webhook"},
	{"soft delete policy", "política de borrado lógico"},
	{"batch size tuning", "ajuste del tamaño de lote"},
	{"secret rotation", "rotación de secretos"},
	{"log redaction", "ocultación de datos en los logs"},
	{"feature rollout by percentage", "despliegue gradual por porcentaje"},
	{"concurrency limit per tenant", "límite de concurrencia por inquilino"},
	{"clock skew between servers", "desfase de reloj entre servidores"},
	{"empty state rendering", "renderizado del estado vacío"},
	{"keyboard focus trap", "trampa de foco de teclado"},
	{"dark mode contrast", "contraste en modo oscuro"},
	{"file name sanitization", "saneamiento de nombres de archivo"},
	{"deadlock between transactions", "interbloqueo entre transacciones"},
	{"backpressure on the queue", "contrapresión en la cola"},
	{"snapshot test churn", "cambios constantes en tests de snapshot"},
}

// technologies are product names, identical in both languages.
var technologies = []string{
	"PostgreSQL", "Redis", "SQLite", "Kafka", "RabbitMQ", "Nginx", "Docker",
	"Kubernetes", "React", "Vue", "Go", "Rust", "TypeScript", "Python", "Django",
	"FastAPI", "Terraform", "Stripe", "Cloudflare", "GraphQL", "gRPC",
	"OpenTelemetry", "Prometheus", "Elasticsearch", "S3",
}

var units = []pair{
	{"seconds", "segundos"}, {"MB", "MB"}, {"retries", "reintentos"},
	{"ms", "ms"}, {"requests", "peticiones"}, {"minutes", "minutos"},
}
