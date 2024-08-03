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

func login() {
	log.Info("Logging into Media CMS")

	url := "http://192.168.1.154/api/v1/whoami"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("Failed to create the request: \n\t", "error", err)
	}

	username := "admin"
	password := "filename7"
	req.SetBasicAuth(username, password)

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Fatal("Failed to get the response: \n\t", "Error", err)
	}

	defer resp.Body.Close()

	log.Info("Response Status: \n\t", "Status Code", resp.Status)

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatal(err)
	}

	// Convert body to JSON
	var data map[string]interface{}
	err = json.Unmarshal(body, &data)

	if err != nil {
		log.Fatal(err)
	}

	username = data["username"].(string)
	log.Info("Logged in as Username: \n\t", "Usernames", username)

}

const watch_dir = "./test/"

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
}

func upload_file(file_path string) {
	url := "http://192.168.1.154/api/v1/media"
	log.Debug("Uploading the file: \n\t", "filename", file_path)

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
	err = writer.WriteField("description", "description - description")
	if err != nil {
		log.Fatalf("Failed to add description field: %v", err)
	}

	// Add the title field to the form
	err = writer.WriteField("title", "title - title")
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

	// Set the Authorization header
	username := "admin"
	password := "filename7"
	req.SetBasicAuth(username, password)

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

	log.Info("Uploading files", "Num File", len(files))

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

	log.Info("Starting the watcher")

	login()
	start_watcher()

	select {}
}
