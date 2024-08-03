package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/errgroup"
)

// Get from env variable WATCH_DIR
var watch_dir = os.Getenv("WATCH_DIR")
var user = os.Getenv("MEDIACMS_USER")
var password = os.Getenv("MEDIACMS_PASSWORD")

func get_files() []string {

	log.Info("Getting the files in the directory: \n\t", watch_dir)

	file_info, err := os.ReadDir(watch_dir)
	if err != nil {
		log.Fatal(err)
	}

	paths := []string{}

	for _, file := range file_info {
		// Skip directories
		if file.IsDir() {
			continue
		}
		full_path := filepath.Join(watch_dir, file.Name())
		paths = append(paths, full_path)
	}

	return paths
}

func move_to_uploaded(file_path string) {
	// Move the file to the uploaded directory (watched directory + _uploaded)
	uploaded_dir := watch_dir + "_uploaded"
	new_path := filepath.Join(uploaded_dir, filepath.Base(file_path))

	// If the uploaded directory does not exist, create it
	if _, err := os.Stat(uploaded_dir); os.IsNotExist(err) {
		err := os.Mkdir(uploaded_dir, 0755)
		if err != nil {
			log.Fatal("Failed to create the uploaded directory: \n\t", "Error", err)
		}
	}

	err := os.Rename(file_path, new_path)
	if err != nil {
		log.Fatal("Failed to move the file: \n\t", "Error", err)
	}

	log.Info("Moved the file to the uploaded directory: \n\t", "Old Path", file_path, "New Path", new_path)
}

func upload_file(file_path string) {
	url := "http://192.168.1.154/api/v1/media"

	filename := filepath.Base(file_path)

	log.Info("Uploading the file: \n\t", "filename", file_path)

	// Create a buffer to hold the multipart form data
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Add the file to the form
	file, err := os.Open(file_path)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("media_file", file_path)
	if err != nil {
		log.Fatalf("Failed to create form file: %v", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		log.Fatalf("Failed to copy file: %v", err)
	}

	// Add the description field to the form
	err = writer.WriteField("description", "Automatically uploaded file from watcher")
	if err != nil {
		log.Fatalf("Failed to add description field: %v", err)
	}

	// Add the title field to the form
	err = writer.WriteField("title", filename)
	if err != nil {
		log.Fatalf("Failed to add title field: %v", err)
	}

	// Close the writer to finalize the multipart form data
	err = writer.Close()
	if err != nil {
		log.Fatalf("Failed to close writer: %v", err)
	}

	// Create a new request
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		log.Fatalf("Failed to create the request: %v", err)
	}

	// Set the Content-Type header to multipart/form-data
	req.Header.Set("Content-Type", writer.FormDataContentType())

	req.SetBasicAuth(user, password)

	// Create a new client
	client := &http.Client{}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to send the request: %v", err)
	}

	// Close the response body
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read the response body: %v", err)
	}

	json_body := map[string]interface{}{}
	err = json.Unmarshal(body, &json_body)
	if err != nil {
		log.Fatalf("Failed to unmarshal the response body: %v", err)
	}

	// Print the response body
	log.Debug("Reponse: \n\t")
	// Print each key-value pair in the response body
	for key, value := range json_body {
		log.Debug("\t", key, value)
	}

	log.Info("Successfully uploaded file: \n\t", "filename", file_path)
}

func tick() {
	log.Info("Ticking the watcher")

	files := get_files()

	log.Info("Found files", "Num Files", len(files))

	upload_g := errgroup.Group{}
	upload_g.SetLimit(3)

	for _, file := range files {
		// Use full system path

		// Limit to max go routines to 3
		upload_g.Go(func() error {
			upload_file(file)
			move_to_uploaded(file)
			return nil
		})
	}

	log.Info("Finished Uploading")

}

func start_watcher() {
	tick()
	ticker := time.NewTicker(5 * time.Second)

	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				tick()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

}

func main() {
	if watch_dir == "" {
		log.Fatal("WATCH_DIR environment variable not set")
	}

	if watch_dir[len(watch_dir)-1] != '/' {
		watch_dir += "/"
	}

	if user == "" {
		log.Fatal("MEDIACMS_USER environment variable not set")
	}

	if password == "" {
		log.Fatal("MEDIACMS_PASSWORD environment variable not set")
	}

	log.Info("Starting the watcher")
	start_watcher()

	select {}
}
