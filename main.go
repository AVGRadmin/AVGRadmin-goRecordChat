package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

//go:embed Recordurbate/recordurbate/bot.py
var botPy []byte

//go:embed Recordurbate/recordurbate/config.py
var configPy []byte

//go:embed Recordurbate/recordurbate/daemon.py
var daemonPy []byte

//go:embed Recordurbate/recordurbate/Recordurbate.py
var recordurbatePy []byte

type Config struct {
	YoutubeDlCmd          string   `json:"youtube-dl_cmd"`
	YoutubeDlConfig       string   `json:"youtube-dl_config"`
	AutoReloadConfig      bool     `json:"auto_reload_config"`
	RateLimit             bool     `json:"rate_limit"`
	RateLimitTime         int      `json:"rate_limit_time"`
	DefaultExportLocation string   `json:"default_export_location"`
	Streamers             []string `json:"streamers"`
}

var defaultConfig = `{
    "youtube-dl_cmd": "youtube-dl",
    "youtube-dl_config": "configs/youtube-dl.config",
    "auto_reload_config": true,
    "rate_limit": true,
    "rate_limit_time": 5,
    "default_export_location": "./list.txt",
    "streamers": []
  }`
var defaultYtDlConfig = `
-o "videos/%(id)s/%(title)s.%(ext)s"
# To reduce output video filesize, use the following instead to limit to [height<1080][fps<?60]
#-f 'best[height<1080][fps<?60]' -o "videos/%(id)s/%(title)s.%(ext)s"
# --quiet
`
var recordingActive bool
var config Config
var configFile = "configs/config.json"
var logFile = "configs/rb.log"

func loadConfig() error {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &config)
}

func saveConfig() error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configFile, data, 0644)
}

func main() {
	_, err := os.ReadDir("configs")
	if err != nil {
		os.Mkdir("configs", 0755)
		os.Create("configs/rb.log")
		os.Create("configs/youtube-dl.config")
		os.Create("configs/config.json")

		err = os.WriteFile("configs/config.json", []byte(defaultConfig), 0755)
		err = os.WriteFile("configs/youtube-dl.config", []byte(defaultYtDlConfig), 0755)
		if err != nil {
			fmt.Println(err)
		}
		_, err = os.ReadDir("configs")
		if err != nil {
			fmt.Println(err)
		}
	}
	// Write embedded Python files to runtime directory
	if err := writeEmbeddedFiles(); err != nil {
		log.Fatalf("Failed to write embedded files: %v", err)
	}

	// Update rootPath to runtime directory
	// Load initial config
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)

	}

	// Create Fyne application
	a := app.New()
	w := a.NewWindow("Streamers Manager")

	// Streamers list
	streamersList := widget.NewList(
		func() int {
			return len(config.Streamers)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Streamer")
		},
		func(i widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(config.Streamers[i])
		},
	)

	// Add streamer
	addStreamerButton := widget.NewButton("Add Streamer", func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("Enter streamer name")

		dialog.ShowForm("Add Streamer", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Streamer", entry),
		}, func(confirmed bool) {
			if confirmed && entry.Text != "" {
				config.Streamers = append(config.Streamers, entry.Text)
				streamersList.Refresh()
				if err := saveConfig(); err != nil {
					dialog.ShowError(err, w)
				}
			}
		}, w)
	})

	// Remove streamer
	removeStreamerButton := widget.NewButton("Remove Streamer", func() {
		selectedIdx := streamersList.Length()
		fmt.Println(selectedIdx)
		if selectedIdx >= 0 && selectedIdx < len(config.Streamers) {
			config.Streamers = append(config.Streamers[:selectedIdx], config.Streamers[selectedIdx+1:]...)
			streamersList.Refresh()
			if err := saveConfig(); err != nil {
				dialog.ShowError(err, w)
			}
		} else {
			dialog.ShowInformation("Error", "No streamer selected", w)
		}
	})

	// Change export location
	changeExportLocationButton := widget.NewButton("Change Export Location", func() {
		entry := widget.NewEntry()
		entry.SetText(config.DefaultExportLocation)

		dialog.ShowForm("Set Export Location", "Save", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Location", entry),
		}, func(confirmed bool) {
			if confirmed {
				config.DefaultExportLocation = entry.Text
				if err := saveConfig(); err != nil {
					dialog.ShowError(err, w)
				}
			}
		}, w)
	})
	// Start app
	startRecordingButton := widget.NewButton("Restart recording", func() {

		if recordingActive {
			return
		}
		if runCMDwithArgs("python3", "Recordurbate.py", "restart") != nil {
			return
		}
		recordingActive = true
	})

	// Layout
	buttons := container.NewVBox(
		addStreamerButton,
		removeStreamerButton,
		changeExportLocationButton,
		startRecordingButton,
	)
	content := container.NewBorder(nil, nil, nil, buttons, streamersList)

	w.SetContent(content)
	w.Resize(fyne.NewSize(500, 400))
	w.ShowAndRun()
}
func runCMDwithArgs(command string, args ...string) error {
	if len(args) > 0 {
		fmt.Println("Script:", args[0])
	}
	cmd := exec.Command(command, args...)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}
	return nil
}

// writeEmbeddedFiles writes all embedded Python files to the filesystem.
func writeEmbeddedFiles() error {
	files := map[string][]byte{
		"bot.py":          botPy,
		"config.py":       configPy,
		"daemon.py":       daemonPy,
		"Recordurbate.py": recordurbatePy,
	}

	for name, content := range files {
		outputPath := name
		if err := os.WriteFile(outputPath, content, fs.ModePerm); err != nil {
			return fmt.Errorf("failed to write %s: %w", name, err)
		}
	}

	return nil
}
