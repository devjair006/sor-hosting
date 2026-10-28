# SOR Hosting — sistema visual

Estado: aprobado por el usuario junto con el diseño del producto. Plataforma: aplicación web administrativa en español. Marca propia: SOR Hosting. Directorio: `sor-hosting`.

## Procedencia

UI/UX Pro Max local: `/home/devjair06/tlya_transportista/.agents/skills/ui-ux-pro-max/SKILL.md`. Consultas ejecutadas: `hosting infrastructure dashboard professional --design-system`, reintento `server administration dashboard --design-system`, `professional technical dashboard --domain typography`, `admin dashboard forms tables --stack shadcn`, `keyboard focus modal --stack shadcn`, `destructive action confirmation --domain ux` y `monitoring time series --domain chart`.

Se adoptan Minimalism & Swiss Style, Fira Sans + Fira Code, diálogos con gestión de foco, tablas claras y gráficas de líneas con etiquetas y alternativa textual. La composición automática Hero + Features + CTA no corresponde a un panel y no se adopta. No se encontró una composición de dashboard verificada en esos dos resultados; la estructura descrita aquí es una propuesta específica del proyecto. La paleta grafito/marfil/esmeralda y la prohibición de degradados proceden del acuerdo con el usuario.

## Identidad

Sólida, precisa, sobria, reconocible. Monograma vectorial de trazos angulares que evoca racks y una ruta ascendente, acompañado por SOR en peso 700 y Hosting en peso 400. No usa logos de Azure, Docker o WordPress como marca propia. Sin fotografías ni ilustraciones genéricas de nubes.

## Tokens iniciales

| Token | Valor | Uso |
|---|---|---|
| `canvas` | `#F7F8F2` | Fondo marfil |
| `surface` | `#FFFFFF` | Tarjetas y diálogos |
| `surface-subtle` | `#EEF1EA` | Cabeceras de tablas y fondos secundarios |
| `ink` | `#17231E` | Texto principal |
| `muted-ink` | `#56645D` | Texto secundario |
| `sidebar` | `#15251E` | Navegación grafito con matiz verde |
| `sidebar-ink` | `#F7F8F2` | Texto de navegación |
| `sidebar-muted` | `#B3C0B6` | Texto secundario de navegación |
| `primary` | `#08734F` | Acción principal y foco sobre claro |
| `on-primary` | `#FFFFFF` | Texto en acción principal |
| `primary-subtle` | `#E4F2E9` | Selección y estados saludables |
| `success-ink` | `#17603F` | Etiqueta de estado saludable |
| `warning-ink` | `#875413` | Etiqueta de aviso |
| `warning-subtle` | `#FFF4D8` | Fondo de aviso |
| `danger` | `#B73131` | Errores y acciones destructivas |
| `danger-subtle` | `#FDECEC` | Fondo de error |
| `border` | `#DAE1D8` | Separadores decorativos |
| `control-border` | `#7B8B80` | Límite visible de controles |

Verificar pares efectivos al implementar, incluidos hover, focus, disabled y gráficas; un token aislado no acredita accesibilidad. El foco necesita contraste propio en sidebar y superficies claras.

## Tipografía y composición

- Fira Sans 400/500/600/700: texto y encabezados; Fira Code 400/500: datos técnicos y logs. Servir fuentes locales con `font-display: swap`.
- Título de página 28–32 px; sección 18–20; cuerpo 14–16, con inputs de 16 px en móvil; texto auxiliar 12–13. Interlineado normal 1.5. Números tabulares.
- Sidebar 248 px; encabezado 72 px; contenido con padding 32 px de escritorio, 24 en tablet y 16 en móvil. Grilla con gaps 16/24; tarjetas padding 20/24; filas de tabla 52 px.
- Radios 8 px en controles y 12 px en tarjetas. Bordes 1 px. Sombras leves solo para diálogos y menús. Densidad alta legible, sin convertir todo en tarjetas.
- Desktop ≥1024 px: sidebar completa. Tablet: navegación compacta. Móvil: Sheet con foco controlado y listado prioritario; tablas anchas tienen representación resumida por filas con detalle accesible.

## Pantallas

- Login: formulario limpio sobre marfil y panel lateral grafito de marca; en móvil, una columna. Sin datos de infraestructura antes de entrar.
- Resumen: encabezado y «Crear sitio»; cuatro indicadores CPU/RAM/disco/tráfico; gráfica de CPU y memoria; resumen de servicios; tabla de sitios; actividad reciente. Intervalo y última actualización junto a la gráfica.
- Sitios: tabla protagonista, búsqueda y filtros; dominio legible y acciones secundarias en menú. Detalle en rutas propias con enlaces compartibles.
- Contenedores: estado, recursos y acciones autorizadas; logs en panel grafito, monospace y pausa/actualizar.
- Dominios y SSL: estados separados para DNS y certificado, fecha de vencimiento y reintentos contextualizados.
- Instalador: cuatro pasos, resumen persistente, validación y progreso durable por fases.
- Backups: historial y acciones explícitas; restauración exige confirmación del dominio y explica mantenimiento y rollback.

## Componentes e interacción

Componentes de shadcn/ui reales: Button, Input, Label, Table, Dialog, AlertDialog, Sheet, DropdownMenu, Tabs, Badge, Tooltip y Select. Adaptarlos mediante tokens. Una sola familia de iconos: Lucide, normalmente 18/20 px, elementos decorativos con `aria-hidden`.

Una acción primaria por pantalla. Destructivos en rojo y con confirmación. Formularios con label persistente, ayuda, errores vinculados y conservación de valores. Contraseñas permiten pegado y gestor. Al enviar: estado de carga; al fallar: mensaje accionable; al terminar: confirmación textual y actualización del recurso.

Mantener foco y retorno de foco de Dialog/Sheet; Escape donde sea seguro. Controles táctiles de 44 px cuando corresponda. Teclado completo, skip link y navegación semántica.

## Estados y datos

Carga: skeleton estático, sin shimmer degradado. Vacío: mensaje y siguiente acción. Error: causa comprensible y reintento. Conexión caída: estado explícito y fecha del último dato. Ninguna cifra de demostración en producción.

Gráficas de línea sólidas, sin rellenos degradados, con unidades, leyendas, datos consultables por teclado, resumen textual y opción de pausar refresco. Color acompañado de texto, marcador o patrón. Los avisos críticos permanecen visibles; no dependen solo de toasts.

Transiciones breves de opacidad/color, 120–180 ms. `prefers-reduced-motion` elimina movimiento prescindible. Sin GSAP, parallax, glassmorphism ni animación decorativa continua.

## Validación antes de entregar UI

Verificar 375, 768, 1024 y 1440 px, zoom 200 %, contraste de texto 4.5:1, foco visible no tapado, labels y errores accesibles, menús/diálogos por teclado, rutas directas, estados de red y cero degradados en CSS/SVG. Comprobar que todos los controles actúan sobre datos reales o presentan claramente su indisponibilidad.
