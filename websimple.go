package main
import (
    "fmt"
    "bufio"
    "net/http"
    "os"
    "path"
    "log"
    _ "io/ioutil"
    "gopkg.in/yaml.v2"
    "regexp"
    "sort"
    "time"
    "strconv"
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
    keys := make([]string, 0, len(fi))
    for _, sub := range fi {
        if sub.IsDir() {
            log.Print(sub.Name())
            //log.Print("here")
            r.data[sub.Name()] = make(map[string]string)
            //log.Print("herex")

            //subdirPath := RANK_CACHE_DIR + sub.Name() + "/"
            keys = append(keys, sub.Name())

        }
    }

    sort.Strings(keys)
    latest := keys[len(keys)-1]
    subdirPath := RANK_CACHE_DIR + latest + "/"

    subdir, _ := os.Open(subdirPath)
    log.Print(subdir)
    key, _ := subdir.Readdir(0)
    for _, pt := range key {
        fileName := subdirPath + pt.Name()
        //log.Print(fileName)
        content := r.ReadFile(fileName)
        r.data[latest][pt.Name()] = string(content)
    }
    dir.Close()
}

func (r *RankServer) ReadFile(fileName string) string {
    var content string
    content = ""
    fh, _ := os.Open(fileName)
    defer fh.Close()
    bfh := bufio.NewReader(fh)
    filter, _ := regexp.Compile("^\\s*(score|rank):")
    for {
        line, _ := bfh.ReadString('\n')
        if line == "" {
            break
        }
        if filter.MatchString(line) {
            content += line
            //log.Print(line)
        }
    }
    return content
}

func (r *RankServer) run() {
    http.ListenAndServe(":4001", nil)
}

func (r *RankServer) dumpData() string {
    yy, _ := yaml.Marshal(r.data)
    return string(yy)
}

func (r *RankServer) latestData() string {
    keys := make([]string, 0, len(r.data))
    for k, _ := range(r.data) {
        keys = append(keys, k)
    }
    log.Print(keys)
    sort.Strings(keys)
    log.Print(keys)
    latest := keys[len(keys)-1]

    yy, _ := yaml.Marshal(r.data[latest])
    ltime, _ := strconv.Atoi(latest)
    jst, _ := time.LoadLocation("Asia/Tokyo")
    fmt.Println("tz:", jst)
    t := time.Unix(int64(ltime), 0).In(jst)
    log.Print(t)
    st := t.Format(time.UnixDate)
    log.Print(st)
    return latest + "\n" + st + "\n" + string(yy)
}

func (r *RankServer) handler( w http.ResponseWriter, _ *http.Request ) {
    //fmt.Fprint( w, r.dumpData() )
    r.checkData()
    fmt.Fprint( w, r.latestData() )
}


func main() {
    r := &RankServer{}
    r.data = make(map[string]map[string]string)
    http.HandleFunc("/", r.handler)
    r.run()
}
