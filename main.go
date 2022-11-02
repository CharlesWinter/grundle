package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/github"
)

func main() {
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
