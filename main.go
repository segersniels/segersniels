package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
)

const (
	host = "0.0.0.0"
	port = "22"
)

type model struct {
	viewport viewport.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	default:
		return m, nil
	}
}

func (m model) View() string {
	return m.viewport.View()
}

func fetchRemoteContent(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	response, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func fetchLocalContent(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	const width = 78
	vp := viewport.New(width, 24)

	// When running a Bubble Tea app over SSH, you shouldn't use the default
	// lipgloss.NewStyle function.
	// That function will use the color profile from the os.Stdin, which is the
	// server, not the client.
	main := bubbletea.MakeRenderer(s)
	vp.Style = main.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	// Override glamour's default style with a custom one to better support light backgrounds.
	// By default glamour determines the style to use on the terminal based on the
	// terminal's background color (which is the server in this case). This is not possible over SSH, so we have to
	// set the style manually.
	var glamourStyle = func() ansi.StyleConfig {
		noColor := ""
		s := glamour.DarkStyleConfig
		s.Document.StylePrimitive.Color = &noColor
		s.CodeBlock.Chroma.Text.Color = &noColor
		s.CodeBlock.Chroma.Name.Color = &noColor
		return s
	}()

	var content string
	url := os.Getenv("README_URL")

	if url != "" {
		log.Info("Remote README specified", "url", url)

		data, err := fetchRemoteContent(url)
		if err != nil {
			log.Info("Could not fetch remote content, falling back to local README")

			data, err = fetchLocalContent("./README.md")
			if err != nil {
				log.Fatal(err)
			}
		}

		content = string(data)
	} else {
		data, err := fetchLocalContent("./README.md")
		if err != nil {
			log.Fatal(err)
		}

		content = string(data)
	}

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStyles(glamourStyle),
		glamour.WithWordWrap(width),
		glamour.WithPreservedNewLines(),
	)
	str, _ := renderer.Render(content)
	vp.SetContent(str)

	m := model{
		viewport: vp,
	}

	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
			logging.Middleware(),
		),
	)

	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}
