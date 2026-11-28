//go:build eval

package eval

// Sentence pools by type: opening, detail and closing, per language. Slots:
// {comp} {topic} {tech} {n} {unit}.
type pools struct{ open, detail, close []string }

var enPools = map[string]pools{
	"bugfix": {
		[]string{"Fixed {topic} in the {comp}.", "Resolved a bug: {topic} affected the {comp}.", "The {comp} suffered from {topic}; now fixed."},
		[]string{"Root cause was the {tech} layer: the limit of {n} {unit} was too low under load.", "The {tech} client kept state between requests, which broke after {n} {unit}.", "It only reproduced when {tech} was slower than {n} {unit}."},
		[]string{"Added a regression test and a note in the runbook.", "Verified in staging before the release.", "A monitoring alert now warns about a recurrence."},
	},
	"decision": {
		[]string{"Decision: handle {topic} in the {comp} explicitly.", "We agreed to address {topic} for the {comp} with {tech}.", "Chosen approach for the {comp}: tackle {topic} first."},
		[]string{"Rejected a custom queue because {tech} already gives {n} {unit} of headroom.", "Trade-off: simpler code now, a limit of {n} {unit} that we may revisit.", "The team prefers {tech} over rolling our own."},
		[]string{"Revisit if traffic doubles.", "Documented in the architecture notes.", "Owner: the backend team."},
	},
	"architecture": {
		[]string{"The {comp} is built on {tech}.", "Architecture of the {comp}: a thin service over {tech}.", "The {comp} sits behind a boundary backed by {tech}."},
		[]string{"It deals with {topic} at the edge, with a budget of {n} {unit}.", "Reads go through {tech}; writes are serialized to avoid {topic}.", "State lives in {tech} and expires after {n} {unit}."},
		[]string{"Other modules call it only through its public interface.", "No component reads its tables directly.", "Diagram kept in the design folder."},
	},
	"discovery": {
		[]string{"Found {topic} in the {comp}.", "Discovered that the {comp} hits {topic}.", "Investigation: the {comp} shows {topic}."},
		[]string{"It happens after {n} {unit} when {tech} is under pressure.", "{tech} logs showed the pattern every {n} {unit}.", "Profiling pointed at {tech} as the source."},
		[]string{"Not fixed yet; needs a decision.", "Worth a ticket for the next sprint.", "Reproduction steps saved for later."},
	},
	"pattern": {
		[]string{"Pattern for {topic} in the {comp}.", "Reusable approach to {topic}, first used in the {comp}.", "How we deal with {topic} around the {comp}."},
		[]string{"Wrap the {tech} call, cap it at {n} {unit} and log the outcome.", "Keep the handler small, delegate to {tech} and retry at most {n} times.", "A helper isolates {tech} so tests can fake it within {n} {unit}."},
		[]string{"Copy the helper instead of sharing a package.", "Apply it to any new integration.", "Examples live next to the tests."},
	},
	"config": {
		[]string{"{tech} configuration for the {comp}.", "Settings used by the {comp} on {tech}.", "Config note: the {comp} reads its {tech} options at startup."},
		[]string{"Timeout set to {n} {unit}; larger values hid {topic}.", "Pool size and a window of {n} {unit} avoid {topic}.", "The value of {n} {unit} comes from load tests."},
		[]string{"Override it with an environment variable.", "Changing it requires a restart.", "Defaults are safe for development."},
	},
	"preference": {
		[]string{"Preference: the {comp} should use {tech}.", "I prefer {tech} for the {comp}.", "Style note for the {comp}: keep {tech} behind one module."},
		[]string{"Avoid {topic} by keeping changes under {n} {unit} of review time.", "Short functions, explicit names and a budget of {n} {unit}.", "Comments explain why, never what."},
		[]string{"Applies to new code only.", "Ask before changing it.", "Agreed with the whole team."},
	},
}

var esPools = map[string]pools{
	"bugfix": {
		[]string{"Corregido {topic} en {comp}.", "Error resuelto: {topic} afectaba a {comp}.", "{comp} sufría {topic}; ya está corregido."},
		[]string{"La causa raíz estaba en la capa de {tech}: el límite de {n} {unit} era muy bajo con carga.", "El cliente de {tech} guardaba estado entre peticiones y se rompía tras {n} {unit}.", "Solo se reproducía cuando {tech} tardaba más de {n} {unit}."},
		[]string{"Se añadió un test de regresión y una nota en el runbook.", "Verificado en staging antes del lanzamiento.", "Una alerta de monitoreo avisa ahora si vuelve a ocurrir."},
	},
	"decision": {
		[]string{"Decisión: tratar {topic} en {comp} de forma explícita.", "Acordamos abordar {topic} en {comp} con {tech}.", "Enfoque elegido para {comp}: atacar primero {topic}."},
		[]string{"Se descartó una cola propia porque {tech} ya da {n} {unit} de margen.", "Compromiso: código más simple ahora y un límite de {n} {unit} que quizá revisemos.", "El equipo prefiere {tech} antes que construir algo propio."},
		[]string{"Revisar si el tráfico se duplica.", "Documentado en las notas de arquitectura.", "Responsable: el equipo de backend."},
	},
	"architecture": {
		[]string{"{comp} está construido sobre {tech}.", "Arquitectura de {comp}: un servicio delgado sobre {tech}.", "{comp} queda detrás de una frontera respaldada por {tech}."},
		[]string{"Trata {topic} en el borde, con un presupuesto de {n} {unit}.", "Las lecturas pasan por {tech}; las escrituras se serializan para evitar {topic}.", "El estado vive en {tech} y caduca a los {n} {unit}."},
		[]string{"Otros módulos lo llaman solo por su interfaz pública.", "Ningún componente lee sus tablas directamente.", "El diagrama está en la carpeta de diseño."},
	},
	"discovery": {
		[]string{"Hallazgo: {topic} en {comp}.", "Descubrimos que {comp} sufre {topic}.", "Investigación: {comp} muestra {topic}."},
		[]string{"Ocurre tras {n} {unit} cuando {tech} está bajo presión.", "Los logs de {tech} mostraban el patrón cada {n} {unit}.", "El perfilado señaló a {tech} como origen."},
		[]string{"Aún sin corregir; hace falta una decisión.", "Merece un ticket para el próximo sprint.", "Pasos de reproducción guardados para más adelante."},
	},
	"pattern": {
		[]string{"Patrón para {topic} en {comp}.", "Enfoque reutilizable para {topic}, usado primero en {comp}.", "Cómo tratamos {topic} alrededor de {comp}."},
		[]string{"Envolver la llamada a {tech}, limitarla a {n} {unit} y registrar el resultado.", "Handler pequeño, delegar en {tech} y reintentar como mucho {n} veces.", "Un helper aísla {tech} para simularlo en tests dentro de {n} {unit}."},
		[]string{"Copiar el helper en lugar de compartir un paquete.", "Aplicarlo a cualquier integración nueva.", "Los ejemplos están junto a los tests."},
	},
	"config": {
		[]string{"Configuración de {tech} para {comp}.", "Ajustes que usa {comp} sobre {tech}.", "Nota de configuración: {comp} lee sus opciones de {tech} al arrancar."},
		[]string{"Timeout de {n} {unit}; valores mayores ocultaban {topic}.", "El tamaño del pool y una ventana de {n} {unit} evitan {topic}.", "El valor de {n} {unit} sale de pruebas de carga."},
		[]string{"Se sobrescribe con una variable de entorno.", "Cambiarlo exige reiniciar.", "Los valores por defecto son seguros para desarrollo."},
	},
	"preference": {
		[]string{"Preferencia: {comp} debería usar {tech}.", "Prefiero {tech} para {comp}.", "Nota de estilo para {comp}: mantener {tech} detrás de un solo módulo."},
		[]string{"Evitar {topic} con cambios de menos de {n} {unit} de revisión.", "Funciones cortas, nombres explícitos y un presupuesto de {n} {unit}.", "Los comentarios explican el porqué, nunca el qué."},
		[]string{"Solo para código nuevo.", "Consultar antes de cambiarlo.", "Acordado con todo el equipo."},
	},
}

var enTitles = map[string]string{
	"bugfix": "Fix {topic} in {comp}", "decision": "Decision on {topic} for {comp}",
	"architecture": "{comp} architecture with {tech}", "discovery": "Found {topic} in {comp}",
	"pattern": "Pattern for {topic} in {comp}", "config": "{tech} configuration for {comp}",
	"preference": "Preference: {comp} with {tech}",
}

var esTitles = map[string]string{
	"bugfix": "Corrección de {topic} en {comp}", "decision": "Decisión sobre {topic} en {comp}",
	"architecture": "Arquitectura de {comp} con {tech}", "discovery": "Hallazgo: {topic} en {comp}",
	"pattern": "Patrón para {topic} en {comp}", "config": "Configuración de {tech} para {comp}",
	"preference": "Preferencia: {comp} con {tech}",
}
