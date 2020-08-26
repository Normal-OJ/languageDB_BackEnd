package kideval

import (
	"errors"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"main.main/src/db"
	"main.main/src/utils"
)

func execute(speakers []string, files []string) (string, string, error) {
	cmdFolderLoc := os.Getenv("CLANG_CMD_FOLDER")
	chaCache := os.Getenv("CHA_CACHE")

	cmdOpts := []string{"+lzho"}
	for _, speaker := range speakers {
		cmdOpts = append(cmdOpts, "+t*"+speaker)
	}
	for _, file := range files {
		file = filepath.Clean(file)

		if !utils.PathChecker(file) {
			return "", "", errors.New("unallowed path")
		}
		cmdOpts = append(cmdOpts, file)
	}

	var out = utils.RunCmd(cmdFolderLoc+"/kideval", cmdOpts)
	if !strings.Contains(out, "<?xml") {
		return "", "", errors.New(out)
	}

	file := strings.Split(out, "<?xml")[1]
	file = "<?xml" + strings.Split(file, "</Workbook>")[0] + "</Workbook>"

	filename := "kideval" + uuid.NewV4().String() + ".xls"
	ioutil.WriteFile(chaCache+"/"+filename, []byte(file), 0644)

	return filename, file, nil
}

func makeRespone(filename string, file string, indicator []string) map[string][]interface{} {
	data := utils.ExtractXMLInfo([]byte(file))
	ret := make(map[string][]interface{})
	ret["filename"] = []interface{}{filename}

	for _, key := range indicator {
		ret[key] = make([]interface{}, 0)
	}

	for _, row := range data[1:] {
		for index, val := range row {
			key := data[0][index].(string)
			_, ok := ret[key]
			if ok {
				ret[key] = append(ret[key], val)
			}
		}
	}

	for k, v := range ret {
		if k == "filename" {
			continue
		}
		mean, n := utils.Mean(v)
		sd, _ := utils.SD(v)

		ret[k] = []interface{}{mean, sd, float64(n)}
	}

	return ret
}

func makeDetailedRespone(filename string, file string) map[string]interface{} {
	data := utils.ExtractXMLInfo([]byte(file))
	ret := make(map[string]interface{})
	ret["filename"] = filename

	for _, row := range data[1:] {
		for index, val := range row {
			key := data[0][index].(string)
			ret[key] = val
		}
	}

	neededKeys := []string{"CTTR", "n_percentage", "v_percentage", "adj", "adj_percentage",
		"adv_percentage", "conj_percentage", "cl_percentage"}

	for _, key := range neededKeys {
		var val interface{}
		switch key {
		case "CTTR":
			val = ret["FREQ_types"].(float64) / math.Sqrt(ret["FREQ_tokens"].(float64)*2)
		case "adj":
			cmdFolderLoc := os.Getenv("CLANG_CMD_FOLDER")
			chaCache := os.Getenv("CHA_CACHE")
			cmdOpts := []string{"+t%mor +s\"adj|*\"", chaCache + "/" + filename, "+t*CHI +d3 -f"}

			out := utils.RunCmd(cmdFolderLoc+"/freq", cmdOpts)
			file := strings.Split(out, "<?xml")[1]
			file = "<?xml" + strings.Split(file, "</Workbook>")[0] + "</Workbook>"
			data := utils.ExtractXMLInfo([]byte(file))
			val = data[0][12]
		case "n_percentage":
		case "v_percentage":
		case "adj_percentage":
		case "adv_percentage":
		case "conj_percentage":
		case "cl_percentage":
			word := strings.Split(key, "-")[0]
			val = ret[word].(float64) / ret["mor_Words"].(float64)
		}
		ret[key] = val
	}

	return ret
}

type pathRequest struct {
	File      []string
	Speaker   []string
	Indicator []string
}

func getFiles(filename string) []string {
	finfo, _ := os.Stat(filename)
	ret := []string{}

	if finfo.IsDir() {
		files, _ := ioutil.ReadDir(filename)
		for _, file := range files {
			ret = append(ret, getFiles(filename+"/"+file.Name())...)
		}
	} else {
		if strings.HasSuffix(finfo.Name(), ".cha") {
			ret = append(ret, filename)
		}
	}

	return ret
}

// PathKidevalRequestHandler is like what it said :P
func PathKidevalRequestHandler(context *gin.Context) {
	var request pathRequest
	err := context.ShouldBind(&request)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "invalid input"})
		return
	}

	defer func() {
		err := recover()
		if err != nil {
			context.String(http.StatusInternalServerError, "internal server error")
			return
		}
	}()

	files := []string{}

	for index, filename := range request.File {
		request.File[index] = utils.CHADIR + "/" + filename

		_, err := os.Stat(request.File[index])
		if err != nil {
			context.JSON(http.StatusNotFound, gin.H{"message": "file: " + filename + " not found"})
			return
		}

		if !utils.PathChecker(filepath.Clean(request.File[index])) {
			context.JSON(http.StatusBadRequest, gin.H{"message": "unallowed path"})
			return
		}

		files = append(files, getFiles(request.File[index])...)
	}

	name, out, err := execute(request.Speaker, files)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	ret := makeRespone(name, out, request.Indicator)

	context.JSON(http.StatusOK, ret)

}

type optionRequest struct {
	Ages      [][]int
	Sex       []int
	Context   []string
	Speaker   []string
	Indicator []string
}

// OptionKidevalRequestHandler is like what it said :P
func OptionKidevalRequestHandler(context *gin.Context) {
	var request optionRequest
	err := context.ShouldBind(&request)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "invalid input"})
		return
	}

	defer func() {
		err := recover()
		if err != nil {
			context.String(http.StatusInternalServerError, "internal server error")
			return
		}
	}()

	var files = db.QueryChaFiles(request.Ages, request.Sex, request.Context)
	if len(files) == 0 {
		context.JSON(http.StatusNotFound, gin.H{"message": "filtered files' size is 0"})
		return
	}

	name, out, err := execute(request.Speaker, files)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	ret := makeRespone(name, out, request.Indicator)

	context.JSON(http.StatusOK, ret)
}

type uploadRequest struct {
	Speaker   []string
	Indicator []string
}

// UploadKidevalRequestHandler is like what it said :P
func UploadKidevalRequestHandler(context *gin.Context) {
	file, _, err := context.Request.FormFile("file")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "file not found"})
		return
	}

	var request uploadRequest
	err = context.ShouldBind(&request)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "invalid input"})
		return
	}

	defer func() {
		err := recover()
		if err != nil {
			context.String(http.StatusInternalServerError, "internal server error")
			return
		}
	}()

	filename := "/tmp/" + uuid.NewV4().String() + ".cha"

	tmpFile, err := os.Create(filename)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"result": err.Error})
		return
	}

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"result": err.Error})
		return
	}

	name, out, err := execute(request.Speaker, []string{filename})
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	ret := makeRespone(name, out, request.Indicator)
	print(request.Indicator)
	print(request.Speaker)
	os.Remove(filename)

	context.JSON(http.StatusOK, ret)
}

type uploadDetailedRequest struct {
	Speaker []string
}

// UploadDetailedKidevalRequestHandler is like what it said :P
func UploadDetailedKidevalRequestHandler(context *gin.Context) {
	file, _, err := context.Request.FormFile("file")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "file not found"})
		return
	}

	var request uploadDetailedRequest
	err = context.ShouldBind(&request)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "invalid input"})
		return
	}

	defer func() {
		err := recover()
		if err != nil {
			context.String(http.StatusInternalServerError, "internal server error")
			return
		}
	}()

	filename := "/tmp/" + uuid.NewV4().String() + ".cha"

	tmpFile, err := os.Create(filename)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"result": err.Error})
		return
	}

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"result": err.Error})
		return
	}

	name, out, err := execute(request.Speaker, []string{filename})
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	ret := makeDetailedRespone(name, out)
	print(request.Speaker)
	os.Remove(filename)

	context.JSON(http.StatusOK, ret)
}
