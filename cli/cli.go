package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"p2p-chat/node"
	"p2p-chat/protocol"

	"github.com/fatih/color"
)

var (
	colorGreen   = color.New(color.FgGreen).SprintFunc()
	colorCyan    = color.New(color.FgCyan).SprintFunc()
	colorYellow  = color.New(color.FgYellow).SprintFunc()
	colorRed     = color.New(color.FgRed).SprintFunc()
	colorMagenta = color.New(color.FgMagenta).SprintFunc()
	colorGray    = color.New(color.FgWhite).SprintFunc()
)

type CLI struct {
	node *node.Node
}

func NewCLI(n *node.Node) *CLI {
	return &CLI{node: n}
}

func (c *CLI) showWelcome() {
	fmt.Println(colorCyan("╔════════════════════════════════════════╗"))
	fmt.Println(colorCyan("║         P2P Chat - Conectado           ║"))
	fmt.Println(colorCyan("╚════════════════════════════════════════╝"))
	fmt.Printf("%s Tu ID: %s\n", colorGreen("➜"), colorYellow(c.node.ID[:8]))
	fmt.Printf("%s Nick: %s\n", colorGreen("➜"), colorYellow(c.node.Nick))
	fmt.Println(colorGray("───────────────────────────────────────"))
	fmt.Println(colorGray("Comandos disponibles:"))
	fmt.Printf("  %s - Conectar a un peer\n", colorCyan("/connect <ip:port>"))
	fmt.Println(colorGray("Ej: "), colorCyan("/connect 127.0.0.1:5000"))
	fmt.Printf("  %s - Listar peers conectados\n", colorCyan("/peers"))
	fmt.Printf("  %s - Enviar mensaje directo\n", colorCyan("/msg <id> <mensaje>"))
	fmt.Printf("  %s - Enviar al chat global\n", colorCyan("/broadcast <mensaje>"))
	fmt.Printf("  %s - Cambiar nick\n", colorCyan("/nick <nuevoNick>"))
	fmt.Printf("  %s - Mostrar historial de conversación\n", colorCyan("/history"))
	fmt.Printf("  %s - Salir del chat\n", colorCyan("/exit"))
	fmt.Println(colorGray("───────────────────────────────────────\n"))
}

func (c *CLI) Start() {
	c.showWelcome()
	reader := bufio.NewReader(os.Stdin)

	for {
		peers := len(c.node.ListPeers())
		status := colorGreen("●")
		if peers == 0 {
			status = colorRed("●")
		}

		fmt.Print(colorCyan(fmt.Sprintf("[%s %d peers] > ", status, peers)))

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if !strings.HasPrefix(input, "/") {
			msg := c.node.CreateChatMessage(input)
			c.node.RecordHistory(msg)
			c.node.Broadcast(msg, "")
			continue
		}

		c.handleCommand(input)
	}
}

func (c *CLI) handleCommand(line string) {
	parts := strings.Split(line, " ")

	cmd := parts[0]
	args := parts[1:]

	switch cmd {

	case "/connect":
		if len(args) != 1 {
			fmt.Println(colorRed("✗ Uso: /connect ip:port"))
			return
		}
		addr := args[0]
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			fmt.Println(colorRed("✗ Formato inválido: usa ip:port"))
			return
		}
		ip := parts[0]
		port := parts[1]
		if port == c.node.Port && (ip == "127.0.0.1" || ip == "localhost") {
			fmt.Println(colorRed("✗ No puedes conectarte a tu propio puerto"))
			return
		}
		err := c.node.Connect(addr)
		if err != nil {
			fmt.Printf("%s Error al conectar: %s\n", colorRed("✗"), err.Error())
			return
		}

		fmt.Printf("%s Conectando a %s...\n", colorGreen("→"), colorYellow(addr))

	case "/peers":
		peers := c.node.ListPeers()
		if len(peers) == 0 {
			fmt.Println(colorYellow("⚠ No hay peers conectados"))
			return
		}
		fmt.Println(colorGreen("Peers conectados:"))
		for _, p := range peers {
			fmt.Printf("  %s %s\n", colorCyan("●"), colorMagenta(p))
		}

	case "/nick":
		if len(args) == 0 {
			fmt.Println(colorRed("✗ Uso: /nick <nuevoNick>"))
			return
		}
		newNick := strings.Join(args, " ")
		c.node.UpdateNick(newNick)
		fmt.Printf("%s Nick actualizado a %s\n", colorGreen("✓"), colorYellow(newNick))

	case "/msg":
		if len(args) < 2 {
			fmt.Println(colorRed("✗ Uso: /msg <id> <mensaje>"))
			return
		}

		shortID := args[0]
		text := strings.Join(args[1:], " ")

		fullID, ok := c.node.ResolveShortID(shortID)
		if !ok {
			fmt.Println(colorRed("✗ Peer no encontrado"))
			return
		}

		msg := c.node.CreateChatMessage(text)
		msg.ToID = &fullID
		c.node.RecordHistory(msg)
		c.node.SendTo(fullID, msg)

	case "/broadcast":
		if len(args) == 0 {
			fmt.Println(colorRed("✗ Uso: /broadcast <mensaje>"))
			return
		}
		text := strings.Join(args, " ")
		msg := c.node.CreateChatMessage(text)
		c.node.RecordHistory(msg)
		c.node.Broadcast(msg, "")

	case "/history":
		history := c.node.History()
		if len(history) == 0 {
			fmt.Println(colorYellow("⚠ No hay historial todavía"))
			return
		}

		fmt.Println(colorGreen("Historial de conversación:"))
		for _, msg := range history {
			var payload protocol.ChatPayload
			json.Unmarshal(msg.Payload, &payload)

			timestamp := time.Unix(msg.Timestamp, 0).Format("15:04:05")
			direction := ""
			if msg.ToID != nil {
				if msg.FromID == c.node.ID {
					direction = fmt.Sprintf("[DM → %s]", (*msg.ToID)[:6])
				} else {
					direction = fmt.Sprintf("[DM ← %s]", msg.FromNick)
				}
			} else {
				direction = "[CHAT]"
			}

			fmt.Printf("  %s %s %s\n", colorCyan(direction), colorYellow(timestamp), colorGray(payload.Message))
		}

	case "/exit":
		fmt.Println(colorGreen("👋 Saliendo del chat..."))
		os.Exit(0)

	default:
		fmt.Println(colorRed("✗ Comando desconocido: " + cmd))
	}
}

func (c *CLI) ListenEvents() {
	for ev := range c.node.Events {

		switch ev.Type {

		case "chat":
			msg := ev.Data.(protocol.Message)

			var payload protocol.ChatPayload
			json.Unmarshal(msg.Payload, &payload)

			encrypted := ""
			if msg.Encrypted {
				encrypted = " 🔒"
			}

			formattedTime := time.Unix(msg.Timestamp, 0).Format("15:04:05")

			if msg.ToID != nil {
				fmt.Printf("\n%s %s %s%s %s\n",
					colorMagenta("[DM]"),
					colorYellow(formattedTime),
					colorCyan(msg.FromNick),
					encrypted,
					colorGray(payload.Message),
				)
			} else {
				fmt.Printf("\n%s %s %s%s %s\n",
					colorGreen("[CHAT]"),
					colorYellow(formattedTime),
					colorCyan(msg.FromNick),
					encrypted,
					colorGray(payload.Message),
				)
			}
			fmt.Print(colorCyan(fmt.Sprintf("[● %d peers] > ", len(c.node.ListPeers()))))

		case "ack":
			//msgID := ev.Data.(string)
			fmt.Printf("\n%s Mensaje entregado \n", colorGreen("✓"))
			fmt.Print(colorCyan(fmt.Sprintf("[● %d peers] > ", len(c.node.ListPeers()))))

		case "system":
			fmt.Printf("\n%s %s\n", colorYellow("[SYSTEM]"), colorGray(ev.Data))
			fmt.Print(colorCyan(fmt.Sprintf("[● %d peers] > ", len(c.node.ListPeers()))))
		}
	}
}
