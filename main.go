package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"github.com/studio-b12/gowebdav"
)

type (
	dav struct {
		ServerURL string
		Username  string
		Password  string
		Client    *gowebdav.Client
	}
	FileList   map[string]fs.FileInfo
	serverInfo struct {
		ServerURL string `json:"server"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}
	config struct {
		OriServerInfo serverInfo `json:"origin"`
		TarServerInfo serverInfo `json:"target"`
	}
)

func newDAV(ServerURL string, Username string, Password string) *dav {
	return &dav{
		ServerURL: ServerURL,
		Username:  Username,
		Password:  Password,
	}
}

func (d *dav) Connect() {
	d.Client = gowebdav.NewClient(d.ServerURL, d.Username, d.Password)
}

func (d *dav) ReadDir(remoteDir string) ([]fs.FileInfo, error) {
	var err error
	for i := 0; i < 3; i++ {
		files, err := d.Client.ReadDir(remoteDir)
		if err == nil {
			return files, nil
		}
	}
	return nil, err
}

func (d *dav) ListFiles(remoteDir string) (FileList, error) {
	fileList := make(FileList)
	files, err := d.ReadDir(remoteDir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		filePath := remoteDir + file.Name()
		fileList[filePath] = file
		if file.IsDir() {
			subDirList, err := d.ListFiles(filePath + "/")
			if err != nil {
				return nil, err
			}
			for k, v := range subDirList {
				fileList[k] = v
			}
		}
	}
	return fileList, err
}

func inList(list FileList, item string) bool {
	for k := range list {
		if k == item {
			return true
		}
	}
	return false
}

func CompareFiles(oWebDAV *dav, tWebDAV *dav, oriFileList FileList, tarFileList FileList) (addFList []string, updateFList []string, removeFList []string) {
	for oName, oFile := range oriFileList {
		if !inList(tarFileList, oName) {
			addFList = append(addFList, oName)
		} else {
			tFile := tarFileList[oName]
			if oFile.Size() != tFile.Size() || oFile.ModTime().After(tFile.ModTime()) {
				updateFList = append(updateFList, oName)
			}
		}
	}
	for tName := range tarFileList {
		if !inList(oriFileList, tName) {
			removeFList = append(removeFList, tName)
		}
	}
	return addFList, updateFList, removeFList
}

func (d *dav) RemoveFile(remoteFile string) error {
	var err error
	for i := 0; i < 3; i++ {
		err = d.Client.Remove(remoteFile)
		if err == nil {
			return nil
		}
	}
	return err
}

func (d *dav) Read(remoteFile string) ([]byte, error) {
	var err error
	for i := 0; i < 3; i++ {
		data, err := d.Client.Read(remoteFile)
		if err == nil {
			return data, nil
		}
	}
	return nil, err
}

func (d *dav) Write(remoteFile string, data []byte) error {
	var err error
	for i := 0; i < 3; i++ {
		err = d.Client.Write(remoteFile, data, 0644)
		if err == nil {
			return nil
		}
	}
	return err
}

func readConfig() config {
	var isErr bool
	data, err := os.ReadFile("config.json")
	if err != nil {
		isErr = true
	}
	var config config
	if json.Unmarshal(data, &config) != nil {
		isErr = true
	}
	if isErr {
		fmt.Println("Error reading config.json")
		os.Exit(1)
	}
	return config
}

func main() {
	config := readConfig()
	oClient := newDAV(config.OriServerInfo.ServerURL, config.OriServerInfo.Username, config.OriServerInfo.Password)
	oClient.Connect()
	fmt.Println("List files from origin server...")
	oriFileList, err := oClient.ListFiles("/")
	if err != nil {
		fmt.Println("can't read from remote dir...")
		return
	}
	tClient := newDAV(config.TarServerInfo.ServerURL, config.TarServerInfo.Username, config.TarServerInfo.Password)
	tClient.Connect()
	fmt.Println("List files from target server...")
	tarFileList, err := tClient.ListFiles("/")
	if err != nil {
		fmt.Println("can't read from remote dir...")
		return
	}
	addFileList, updateFileList, removeFileList := CompareFiles(oClient, tClient, oriFileList, tarFileList)
	for _, v := range addFileList {
		fmt.Println("add file: " + v)
		data, err := oClient.Read(v)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", v, err)
		} else {
			err := tClient.Write(v, data)
			if err != nil {
				fmt.Printf("Error writing file %s: %v\n", v, err)
			} else {
				fmt.Printf("File %s written successfully\n", v)
			}
		}
	}
	for _, v := range updateFileList {
		fmt.Println("update file: " + v)
		data, err := oClient.Read(v)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", v, err)
		} else {
			err := tClient.Write(v, data)
			if err != nil {
				fmt.Printf("Error writing file %s: %v\n", v, err)
			} else {
				fmt.Printf("File %s written successfully\n", v)
			}
		}
	}
	for _, v := range removeFileList {
		fmt.Println("remove file: " + v)
		err := tClient.RemoveFile(v)
		if err != nil {
			fmt.Printf("Error removing file %s: %v\n", v, err)
		} else {
			fmt.Printf("File %s removed successfully\n", v)
		}
	}
}
