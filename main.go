package main

import (
        "fmt"
        "flag"
        "strings"
	"net/http"
        "io/ioutil"
        "encoding/json"
        "os"
        "os/exec"
        "path"
)

type GithubWebhook struct {
        Ref string `json:"ref"`
        Repository *GithubRepository `json:"repository"`
        HeadCommit *GithubCommit `json:"head_commit"`
}

type GithubRepository struct {
        Url string `json:"url"`
}

type GithubCommit struct {
        Id string `json:"id"`
}

func main() {
        fmt.Println("Starting up")
	port := flag.String("port", "9001", "Port")

        cwd, err := os.Getwd()
        if err != nil {
            fmt.Println(err)
        }

        commitDir := path.Join(cwd, "commits")


	handler := func(w http.ResponseWriter, r *http.Request) {

		pathStr := r.URL.Path

                if strings.HasPrefix(pathStr, "/webhook") {
                        body, err := ioutil.ReadAll(r.Body)
                        if err != nil {
                                fmt.Println(err)
                                w.WriteHeader(400)
                                fmt.Fprintf(w, "%s", err)
                                return
                        }

                        webhook := &GithubWebhook{}
                        err = json.Unmarshal(body, &webhook)
                        if err != nil {
                                fmt.Println(err)
                                w.WriteHeader(400)
                                fmt.Fprintf(w, "%s", err)
                                return
                        }

                        fmt.Println(webhook.HeadCommit.Id)
                        fmt.Println(webhook.Repository.Url)

                        srcDir := path.Join(commitDir, webhook.HeadCommit.Id, "src")

                        cloneCommand := exec.Command("git", "clone", webhook.Repository.Url, srcDir)
                        _, err = cloneCommand.Output()
                        if err != nil {
                                fmt.Println(err)
                                w.WriteHeader(400)
                                fmt.Fprintf(w, "%s", err)
                                return
                        }


                        fmt.Println(webhook.Ref)
                        args := []string{"-C", srcDir, "checkout", webhook.HeadCommit.Id}
                        fmt.Println(args)
                        checkoutCommand := exec.Command("git", args...)
                        _, err = checkoutCommand.Output()
                        if err != nil {
                                fmt.Println(err.(*exec.ExitError).Stderr)
                                w.WriteHeader(400)
                                return
                        }

                        buildDir := path.Join(commitDir, webhook.HeadCommit.Id, "build")
                        mkdirCommand := exec.Command("mkdir", "-p", buildDir)
                        _, err = mkdirCommand.Output()
                        if err != nil {
                                fmt.Println(err.(*exec.ExitError).Stderr)
                                w.WriteHeader(400)
                                return
                        }


                        srcMount := fmt.Sprintf("type=bind,source=%s,target=/src", srcDir)
                        buildMount := fmt.Sprintf("type=bind,source=%s,target=/build", buildDir)
                        args = []string{"run", "--rm", "-i", "--mount", srcMount, "--mount", buildMount, "bam.iobio", "/src/build.sh"}
                        fmt.Println(args)
                        buildCommand := exec.Command("docker", args...)
                        //buildCommand.Dir = outDir
                        _, err = buildCommand.Output()
                        if err != nil {
                                fmt.Println(err)
                                fmt.Println(err.(*exec.ExitError).Stderr)
                                w.WriteHeader(400)
                                return
                        }

                        w.Write([]byte("Webhook"))
                } else {

                        hostParts := strings.Split(r.Host, ".")
                        sha := hostParts[0]

                        rootDir := path.Join(commitDir, sha, "build")
                        fmt.Println(rootDir)

                        if _, err := os.Stat(rootDir); !os.IsNotExist(err) {
                                filePath := path.Join(rootDir, r.URL.Path)
                                fmt.Println(r.URL.Path)
                                fmt.Println(filePath)

                                _, err := os.Stat(filePath)
                                if r.URL.Path == "/" || os.IsNotExist(err) {
                                        indexPath := path.Join(rootDir, "index.html")
                                        file, err := ioutil.ReadFile(indexPath)
                                        if err != nil {
                                                fmt.Println(err)
                                                fmt.Println(err.(*exec.ExitError).Stderr)
                                                w.WriteHeader(400)
                                                return
                                        }

                                        w.Write(file)
                                } else {
                                        file, err := ioutil.ReadFile(filePath)
                                        if err != nil {
                                                fmt.Println(err)
                                                fmt.Println(err.(*exec.ExitError).Stderr)
                                                w.WriteHeader(400)
                                                return
                                        }

                                        w.Write(file)
                                }
                        } else {
                                w.Write([]byte("does not exist"))
                        }
                }
        }

        err = http.ListenAndServe(":"+*port, http.HandlerFunc(handler))
	if err != nil {
		fmt.Println(err)
	}
}
