# p2p-chat

Chat CLI P2P en Go con encriptación y handshake firmado para mitigar spoofing.

## 📌 Descripción

Proyecto de chat punto a punto construido en Go. Incluye cifrado AES-256-GCM (hop-by-hop), intercambio de claves ECDH y autenticación de handshakes mediante firmas (`ed25519`). El objetivo es ofrecer una base modular para explorar networking, criptografía y diseño P2P.

## ✨ Características (destacadas)

- ✅ **Handshakes firmados** — cada handshake incluye una clave pública de firma y una firma `ed25519` que permite validar la identidad del peer.
- ✅ **Encriptación de transporte** — AES-256-GCM con claves derivadas por ECDH P-256 por enlace.
- ✅ **Confirmación de entrega (ACK)** — ACKs para mensajes directos.
- ✅ **CLI interactiva** — colores, indicadores y comandos útiles.
- ✅ **Descubrimiento de peers** — propagación controlada de peers conocidos (se guarda `knownPeers` con nick y public key).

## 🧱 Estructura del proyecto

- `main.go`: inicializa el nodo y la CLI.
- `node/`: lógica P2P, manejo de peers, eventos, encriptación y retransmisión.
- `peer/`: representa conexión TCP y loop de lectura/escritura.
- `protocol/`: esquema de mensajes JSON y payloads (chat, handshake, ack).
- `crypto/`: ECDH P-256, AES-GCM y utilidades para firmas `ed25519`.
- `cli/`: interfaz de línea de comandos.

## 🔧 Componentes clave

### Node

- `Node.PrivateKey` (ECDH P-256) y `Node.PublicKey` (hex)
- `Node.SignPrivateKey` y `Node.SignPublicKey` (`ed25519`, public key en base64)
- `Node.AddPeer(p)` / `Node.RemovePeer(id)` / `Node.ListPeers()`

### Peer

- `Peer` contiene `Conn`, `SharedKey` (AES) y `SignPublicKey` del peer remoto.

### Protocol

`protocol.HandshakePayload` ahora incluye `SignPublicKey` y `Signature`.

Tipos de mensaje:
- `handshake`: intercambio de identidad y clave pública + firma
- `chat`: mensaje de texto (encriptado hop-by-hop para mensajes directos)
- `ack`: confirmación de entrega

## 🔐 Autenticación de handshakes

- El handshake se firma con `ed25519` usando la clave de firma local (clave pública enviada en `SignPublicKey`).
- Al recibir un handshake, el nodo verifica la firma sobre los campos del handshake antes de aceptar la conexión y propagar rutas.
- Esto mitiga spoofing y poisoning simples, pero no sustituye a una infraestructura de confianza (PKI) para garantizar identidad a largo plazo.

## 🚀 Flujo resumido

1. Nodo genera par ECDH y par de firma `ed25519`.
2. Al conectar, se envía `handshake` firmado con `SignPublicKey` y `Signature`.
3. Receptor valida la firma y deriva la clave compartida ECDH para cifrar el canal directo.
4. Mensajes directos se encriptan con AES-256-GCM usando la clave derivada.
5. ACKs se usan para confirmar entregas directas.

## 💬 Comandos disponibles

- `/connect <ip:port>` — conectar a un nodo
- `/peers` — listar peers directos e indirectos (indicando si son indirectos y vía quién)
- `/msg <shortID> <texto>` — enviar mensaje directo (encriptado)
- `/broadcast <texto>` — enviar mensaje a todos
- `/history` — mostrar historial local

## 📊 Ejemplo de ejecución

Terminal 1:
```bash
go run . 5000 Alice
```

Terminal 2:
```bash
go run . 5001 Bob
/connect 127.0.0.1:5000
/peers
# ●  Alice 🔒
/msg a Hola desde Bob
# ✓ Mensaje entregado
```

## 🔎 Seguridad y limitaciones actuales

- Las firmas en handshakes autenticán el origen del handshake, pero **no** hay un anclaje de confianza (PKI/CA) por lo que nodos maliciosos con claves propias siguen pudiendo participar.
- El cifrado de mensajes es hop-by-hop entre nodos remotos; los nodos intermedios pueden ver la carga si se reenvía descifrada.
- `knownPeers` se propaga y se almacena, lo que puede permitir envenenamiento de rutas si no se aplica política adicional.
- No hay rate limiting ni límites de conexión por ahora; añadirlos es recomendable contra DoS.

## ✅ Buenas prácticas recomendadas

- Establecer una lista de nodos de confianza (whitelist) o usar un canal de distribución seguro de claves públicas.
- Añadir rate limiting por IP/peer y límites máximos de conexiones simultáneas.
- Validar y acotar el tamaño de `knownPeers` propagado para evitar crecimiento ilimitado.
- Considerar cifrado E2E verdadero para mensajes privados si la privacidad es crítica.

## 📦 Dependencias

```
github.com/google/uuid v1.6.0
github.com/fatih/color v1.17.0
```

Instalar:
```bash
go mod download
go get github.com/fatih/color@v1.17.0
```

## 📌 Próximos pasos sugeridos

1. Implementar limits y rate-limiting por peer/IP.
2. Evaluar modelo de confianza: PKI, notary service o intercambio fuera de banda de `SignPublicKey`.
3. Considerar cifrado E2E para mensajes directos (no sólo hop-by-hop).

## 🎯 Objetivo

Proveer una base segura y modular para experimentar con protocolos P2P reales y evolucionar hacia un sistema con identidad verificable y resistencia a ataques de red.
