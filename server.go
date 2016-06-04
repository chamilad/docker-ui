package main

import (
        "net/http"
        "io/ioutil"
        "log"
        "crypto/x509"
        "crypto/tls"
        "errors"
        "encoding/json"
        "html/template"
        "fmt"
        "os"
        "strconv"
        "encoding/base64"
)

var (
        serverPort = os.Getenv("DOCKERUI_PORT")
        dockerApi = "https://" + os.Getenv("DOCKERUI_API_HOST") + "/v2/"
        skipApiVerification, _ = strconv.ParseBool(os.Getenv("DOCKERUI_API_SKIP_VERIFICATION"))
        cacertPath = os.Getenv("DOCKERUI_API_CACRT")
        // TODO: get this ep from auth header
        authEndpoint = "https://" + os.Getenv("DOCKERUI_API_HOST") + ":5001/auth"
        authService = "Docker registry"
        imageListScope = "registry:catalog:*"
        authUsername = os.Getenv("DOCKERUI_USERNAME")
        authPassword = os.Getenv("DOCKERUI_PASSWORD")

        htmlTemplates = template.Must(template.ParseFiles("tmpl/index.html", "tmpl/tags.html"))
)

type Token struct {
        Token string `json:"token"`
}

type TagList struct {
        Name string `json:"name"`
        Tags []string `json:"tags"`
}

type RepoList struct {
        Repos []string `json:"repositories"`
}

// Creates a client that verifies API Certificate with a given CA certificate
func createCertValidatingClient() *http.Client {
        cacert, err := ioutil.ReadFile(cacertPath)
        if err != nil {
                log.Fatal("Error: Couldn't read CA Cert file")
        }

        caCertPool := x509.NewCertPool()
        caCertPool.AppendCertsFromPEM(cacert)

        tlsConfig := &tls.Config{
                RootCAs: caCertPool,
        }

        tlsConfig.BuildNameToCertificate()
        transport := &http.Transport{TLSClientConfig: tlsConfig}
        client := &http.Client{Transport: transport}

        return client
}

// Creates a client that skip API SSL verification
func createInsecureClient() *http.Client {
        insecureTransport := &http.Transport{
                TLSClientConfig:&tls.Config{InsecureSkipVerify: true},
        }

        client := &http.Client{Transport: insecureTransport}

        return client
}

func getDockerApiClient(authScope string) (*http.Client, string) {
        var client *http.Client
        if skipApiVerification == true {
                client = createInsecureClient()
        } else {
                client = createCertValidatingClient()
        }

        token, err := auth(authScope, client)
        if err != nil {
                return nil, ""
        }

        return client, token
}

func auth(authScope string, client *http.Client) (string, error) {
        //client := createCertValidatingClient()
        authReq, err := http.NewRequest("GET", authEndpoint + "?service=" + authService + "&scope=" + authScope, nil)
        q := authReq.URL.Query()
        authReq.URL.RawQuery = q.Encode()
        //log.Println(authReq.URL.String())
        if err != nil {
                return "", err
        }

        b64AuthString := base64.StdEncoding.EncodeToString([]byte(authUsername + ":" + authPassword))
        authReq.Header.Set("Authorization", "Basic " + b64AuthString)
        resp, err := client.Do(authReq)
        if err != nil {
                log.Println("Error when requesting auth... : ", err)
                return "", err
        }

        if (resp.StatusCode == 200) {
                t := new(Token)
                respBody, err := ioutil.ReadAll(resp.Body)
                if err != nil {
                        return "", err
                }

                err = json.Unmarshal(respBody, &t)
                if err != nil {
                        return "", err
                }

                //log.Println("Token: ", t.Token)
                // TODO: validate token later
                return t.Token, nil
        }

        //log.Println("Status code: ", resp)
        return "", errors.New("Token request failed.")
}

func getTagList(img string) (*TagList, error) {
        endpoint := dockerApi + img + "/tags/list"
        client, token := getDockerApiClient("repository:" + img + ":pull")

        getTagsReq, err := http.NewRequest("GET", endpoint, nil)
        if err != nil {
                return nil, err
        }

        getTagsReq.Header.Set("Authorization", "Bearer " + token)
        resp, err := client.Do(getTagsReq)
        respBody, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                return nil, err
        }

        //log.Println("tags: ", string(respBody))
        taglistResp := new(TagList)
        err = json.Unmarshal(respBody, &taglistResp)
        if err != nil {
                return nil, err
        }

        return taglistResp, nil
}

// https://github.com/docker/distribution/blob/master/docs/spec/api.md#listing-repositories
func getRepoList() (*RepoList, error) {
        endpoint := dockerApi + "_catalog"
        client, token := getDockerApiClient(imageListScope)

        getReposReq, err := http.NewRequest("GET", endpoint, nil)
        if err != nil {
                return nil, err
        }

        getReposReq.Header.Set("Authorization", "Bearer " + token)
        //log.Println(getReposReq.URL.String())
        resp, err := client.Do(getReposReq)
        respBody, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                return nil, err
        }

        //log.Println("repositories: ", string(respBody))
        repoListResp := new(RepoList)
        err = json.Unmarshal(respBody, &repoListResp)
        if err != nil {
                return nil, err
        }

        return repoListResp, nil
}

func repoListHandler(w http.ResponseWriter, r *http.Request) {
        repos, err := getRepoList()
        if err != nil {
                return
        }

        err = htmlTemplates.ExecuteTemplate(w, "index.html", repos)
        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }
}

func repoInfoHandler(w http.ResponseWriter, r *http.Request) {
        imageName := r.URL.Path[len("/i/"):]
        tags, err := getTagList(imageName)
        if err != nil {
                return
        }

        w.Header().Set("Content-type", "text/html")

        err = htmlTemplates.ExecuteTemplate(w, "tags.html", tags)
        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }
}

func main() {
        if serverPort == "" {
                serverPort = "8080"
        }

        http.HandleFunc("/", repoListHandler)
        http.HandleFunc("/i/", repoInfoHandler)
        //http.Handle("/res/", http.StripPrefix("/res/", http.FileServer(http.Dir("./tmpl/res"))))
        http.Handle("/r/", http.StripPrefix("/r/", http.FileServer(http.Dir("tmpl/res"))))
        fmt.Println("Serving on port " + serverPort + "...")
        err := http.ListenAndServe(":" + serverPort, nil)
        if err != nil {
                fmt.Println("Fatal error when starting HTTP Server: ", err)
        }
}
