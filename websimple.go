package main
import (
    "fmt"
    "bufio"
    "net/http"
    "os"
    "path"
    "log"
    "io/ioutil"
    "gopkg.in/yaml.v2"
    "regexp"
    "sort"
    "time"
    "strconv"
)


var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"

type RankServer struct {
    //data map[string]map[string]string
    data map[string][]map[int]int
}

func (r *RankServer) checkData() {
    dir, err := os.Open(RANK_CACHE_DIR)
    if err != nil {
        log.Fatal(err)
    }
    log.Print(dir)

    fi, _ := dir.Readdir(0)
    all_timestamp := make([]string, 0, len(fi))
    // sub: dir name 1467555420
    for _, sub := range fi {
        if sub.IsDir() {
            timestamp := sub.Name()
            log.Print(timestamp)
            r.data[timestamp] = make([]map[int]int, 2)
            r.data[timestamp][0] = make(map[int]int)
            r.data[timestamp][1] = make(map[int]int)

            //subdirPath := RANK_CACHE_DIR + sub.Name() + "/"
            all_timestamp = append(all_timestamp, timestamp)

        }
    }

    sort.Strings(all_timestamp)
    latest := all_timestamp[len(all_timestamp)-1]
    subdirPath := RANK_CACHE_DIR + latest + "/"

    subdir, _ := os.Open(subdirPath)
    log.Print(subdir)
    key, _ := subdir.Readdir(0)
    for _, pt := range key {
        rankingType := r.RankingType(pt.Name())
        fileName := subdirPath + pt.Name()
        //log.Print(fileName)
        content, _ := ioutil.ReadFile(fileName)

        var local_rank_list []map[string]interface{}
        yaml.Unmarshal(content, &local_rank_list)

        if len(local_rank_list) > 0 {
            rank := local_rank_list[0]["rank"].(int)
            score := local_rank_list[0]["score"].(int)
            r.data[latest][rankingType][rank] = score
        } else {
            rank := r.FilenameToRank(pt.Name())
            r.data[latest][rankingType][rank] = 0
        }
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

func (r *RankServer) RankingType(fileName string) int {
    filter, _ := regexp.Compile("r01\\.\\d+$")
    if filter.MatchString(fileName) {
        // event pt
        return 0 // r01.xxxxxx
    } else {
        // high score
        return 1 // r02.xxxxxx
    }
}

func (r *RankServer) FilenameToRank(fileName string) int {
    //log.Print("fileName", fileName)
    filter, _ := regexp.Compile("r\\d{2}\\.(\\d+)$")
    submatch := filter.FindStringSubmatch(fileName)
    n, _ := strconv.Atoi(submatch[1])
    //log.Print("fileName", fileName, "n", n, "submatch", submatch)
    return (n - 1) * 10 + 1
}


func (r *RankServer) run() {
    http.ListenAndServe(":4001", nil)
}

func (r *RankServer) dumpData() string {
    yy, _ := yaml.Marshal(r.data)
    return string(yy)
}

func (r *RankServer) latestData() string {
    all_timestamp := make([]string, 0, len(r.data))
    for k, _ := range(r.data) {
        all_timestamp = append(all_timestamp, k)
    }
    log.Print(all_timestamp)
    sort.Strings(all_timestamp)
    log.Print(all_timestamp)
    latest := all_timestamp[len(all_timestamp)-1]

    yy, _ := yaml.Marshal(r.data[latest])
    ltime, _ := strconv.Atoi(latest)
    jst, _ := time.LoadLocation("Asia/Tokyo")
    log.Print("tz:", jst)
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
    r.data = make(map[string][]map[int]int)
    http.HandleFunc("/", r.handler)
    r.run()
}
