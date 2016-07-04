package main
import (
    "fmt"
    "net/http"
    "os"
    "path"
    "log"
    "io/ioutil"
    "gopkg.in/yaml.v2"
)

var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"

type RankServer struct {
    data map[string]map[string]string
}

func (r *RankServer) checkData() {
    dir, err := os.Open(RANK_CACHE_DIR)
    if err != nil {
        log.Fatal(err)
    }
    log.Print(dir)

    fi, _ := dir.Readdir(0)
    for _, sub := range fi {
        if sub.IsDir() {
            log.Print(sub.Name())
            log.Print("here")
            r.data[sub.Name()] = make(map[string]string)
            log.Print("herex")

            subdirPath := RANK_CACHE_DIR + sub.Name() + "/"
            subdir, _ := os.Open(RANK_CACHE_DIR + sub.Name())
            log.Print(subdir)
            key, _ := subdir.Readdir(0)
            for _, pt := range key {
                fileName := subdirPath + pt.Name()
                log.Print(fileName)
                content, _ := ioutil.ReadFile(fileName)
                r.data[sub.Name()][pt.Name()] = string(content)
            }
        }
    }
    dir.Close()
}

func (r *RankServer) run() {
    http.ListenAndServe("127.0.0.1:4001", nil)
}

func (r *RankServer) dumpData() string {
    yy, _ := yaml.Marshal(r.data)
    return string(yy)
}

func (r *RankServer) handler( w http.ResponseWriter, _ *http.Request ) {
    fmt.Fprint( w, r.dumpData() )
}


func main() {
    r := &RankServer{}
    r.data = make(map[string]map[string]string)
    http.HandleFunc("/", r.handler)
    r.checkData()
    r.run()
}
