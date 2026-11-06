# P2P-chat

Chat CLI P2P en Go con cifrado por enlace, handshakes firmados y un modelo de enrutamiento básico para experimentos de red distribuida.

## Estado actual del sistema

El proyecto ya funciona como un prototipo funcional de red P2P con:

- Conexiones TCP directas entre nodos.
- Generación de claves ECDH P-256 y pares de firma ed25519 por nodo.
- Handshakes firmados para validar la identidad del peer antes de aceptar la sesión.
- Derivación de una clave compartida por enlace para cifrar mensajes directos con AES-256-GCM.
- Enrutamiento básico de mensajes, propagación limitada de peers conocidos y ACKs para confirmación de entrega.
- CLI interactiva con comandos para conectar, listar peers, enviar mensajes directos o globales, cambiar nick y consultar historial.

## Qué está implementado hoy

### Red y conexión

- El nodo escucha en un puerto TCP y acepta conexiones entrantes.
- Se pueden abrir conexiones salientes con `/connect <ip:port>`.
- Cada peer se registra en el nodo y se almacena en un mapa interno de peers activos.
- Se aplica un límite máximo de peers conectados y un esquema simple de rate limiting por conexión.

### Criptografía

- Se usan claves ECDH P-256 para derivar una clave compartida por enlace.
- Se usan firmas ed25519 sobre el payload del handshake para reducir spoofing y poisoning simples.
- Los mensajes directos pueden cifrarse con AES-256-GCM cuando existe una clave compartida con el peer remoto.
- La identidad del nodo (ID y claves privadas) se persiste en `identity.json` y se reutiliza entre reinicios.
- La ruta de identidad puede cambiarse con la variable de entorno `P2P_CHAT_IDENTITY_FILE`.

### Mensajería

- Los mensajes de tipo `chat` soportan difusión global y mensajes dirigidos a un peer específico.
- El sistema usa TTL para evitar retransmisiones infinitas.
- Los ACKs se envían cuando un mensaje dirigido llega a su destinatario.
- El nodo guarda un historial local de mensajes y expone comandos para consultarlo.

### CLI

Comandos disponibles actualmente:

- `/connect <ip:port>` — conectar a otro nodo.
- `/peers` — listar peers directos e indirectos conocidos.
- `/msg <shortID> <mensaje>` — enviar un mensaje directo.
- `/broadcast <mensaje>` — enviar un mensaje a todos los peers conectados.
- `/nick <nuevoNick>` — actualizar el nick del nodo.
- `/history` — mostrar el historial local.
- `/exit` — salir del chat.

## Estructura del proyecto

- `main.go`: inicializa el nodo y arranca la CLI.
- `node/`: lógica central del P2P, gestión de peers, eventos, enrutamiento, historial y retransmisión.
- `peer/`: representa una conexión TCP y su loop de lectura/escritura.
- `protocol/`: definiciones de mensajes JSON (`handshake`, `chat`, `ack`) y payloads.
- `crypto/`: utilidades para ECDH, AES-GCM y firmas ed25519.
- `cli/`: interfaz de línea de comandos.
- `transport/`: directorio reservado para futuras abstracciones de transporte.

## Flujo de funcionamiento

1. El nodo genera un par ECDH y un par de firma ed25519.
2. Al conectar, se intercambia un `handshake` con `SignPublicKey` y `Signature`.
3. El receptor valida la firma, deriva la clave compartida y prepara el peer para mensajes cifrados por enlace.
4. Los mensajes directos se cifran con AES-256-GCM usando esa clave compartida.
5. Los mensajes globales o reencaminados usan TTL y se propagan de forma controlada.

## Limitaciones actuales

El proyecto sigue siendo un prototipo y tiene varias limitaciones importantes:

- No hay una infraestructura de confianza real (PKI/CA); las firmas de handshake ayudan, pero no resuelven identidad verificable a largo plazo.
- El cifrado actual es hop-by-hop por enlace, no end-to-end para mensajes privados.
- El enrutamiento es básico y depende de la propagación de peers conocidos.
- No hay almacenamiento persistente ni recuperación de estado tras reinicio.
- No hay tests automatizados ni cobertura de integración en el repositorio.
- La carpeta `transport/` está vacía y está preparada para una abstracción más limpia del transporte.

## Dependencias

- `github.com/google/uuid`
- `github.com/fatih/color`

Instalación:

```bash
go mod download
```

## Cómo ejecutarlo

Terminal 1:

```bash
go run . 5000 Alice
```

Terminal 2:

```bash
go run . 5001 Bob
/connect 127.0.0.1:5000
/peers
/msg <id> Hola desde Bob
```
