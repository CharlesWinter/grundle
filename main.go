package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-github/github"
)

type model struct {
	choices  []string         // items on the to-do list
	cursor   int              // which to-do list item our cursor is pointing at
	selected map[int]struct{} // which to-do items are selected
}

func initialModel() model {
	return model{
		// Our to-do list is a grocery list
		choices: []string{"Add New Package", "Update Existing Package", "Remove Old Package"},

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected: make(map[int]struct{}),
	}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	// The header
	s := "What would you like to do?\n\n"

	// Iterate over our choices
	for i, choice := range m.choices {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Is this choice selected?
		checked := " " // not selected
		if _, ok := m.selected[i]; ok {
			checked = "x" // selected!
		}

		// Render the row
		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	client := github.NewClient(nil)

	repoName := "helix"

	opt := &github.ListOptions{Page: 1, PerPage: 2}
	releases, _, err := client.Repositories.ListReleases(context.Background(), "helix-editor", repoName, opt)

	if err != nil {
		fmt.Println(err)
	}

	targetRelease := releases[1]

	if targetRelease == nil {
		panic("cannot get latest release")
	}

	var tagName = targetRelease.GetTagName()

	var hasAppImage bool
	var downloadURL string
	for _, releaseAsset := range targetRelease.Assets {
		if strings.HasSuffix(releaseAsset.GetBrowserDownloadURL(), "AppImage") {
			hasAppImage = true
			downloadURL = releaseAsset.GetBrowserDownloadURL()
			fmt.Println("browser download url is", releaseAsset.GetBrowserDownloadURL())
		}
	}

	if !hasAppImage {
		fmt.Println("package has no AppImage. Goodbye!")
		os.Exit(0)
	}

	// download the file
	filePath := fmt.Sprintf("%s/.grundle/packages/%s/%s.%s", os.Getenv("HOME"), repoName, repoName, tagName)
	fmt.Println("filePath is", filePath)
	err = downloadFile(filePath, downloadURL)
	if err != nil {
		panic(err)
	}

	// make it (very!) permissive
	os.Chmod(filePath, 0777)

	binPath := fmt.Sprintf("%s/.grundle/bin/%s", os.Getenv("HOME"), repoName)
	if err := upsertSymlink(filePath, binPath); err != nil {
		panic(err)
	}
}

func upsertSymlink(src, dst string) error {
	// sack off the old symlink path
	if _, err := os.Lstat(dst); err == nil {
		os.Remove(dst)
	}

	// symlink it to the binaries folder
	if err := os.Symlink(src, dst); err != nil {
		return err
	}
	return nil
}

func downloadFile(filepath string, url string) (err error) {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
