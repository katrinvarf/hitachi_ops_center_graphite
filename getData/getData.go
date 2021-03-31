package getData

import(
	"github.com/sirupsen/logrus"
	"github.com/katrinvarf/hitachi_graphite/config"
	"github.com/katrinvarf/hitachi_graphite/sendData"
	//"../config"
	//"../sendData"
	"net/http"
	"encoding/csv"
	"encoding/json"
	"strings"
	"strconv"
	"errors"
	"time"
	//"fmt"
	"io/ioutil"
	"regexp"
	"bytes"
)

var (
	num_attempts = 2
	period_attempts = 60
	interval_fix = int64(180)
)

type TInfoColumn struct {
	index int
	dataType string
}

type TStorageApi struct {
	InstanceName string
	HostName string
}


func GetAgents(log *logrus.Logger, api config.TApiTuningManager)(map[string]TStorageApi, error){
	url := api.Protocol + "://" + api.Host + ":" + api.Port + "/TuningManager/v1/objects/AgentForRAID"
	data_byte, err := getDataFromApi(log, url, api.User, api.Password)
	if err!=nil{
		log.Debug("Failed to get data AgentForRAID from api: Error: ", err)
		return nil, err
	}

	var target interface{}
	json.NewDecoder(bytes.NewReader(data_byte)).Decode(&target)
	res_data := make(map[string]TStorageApi)
	for _, item := range target.(map[string]interface{})["items"].([]interface{}){
		if item.(map[string]interface{})["storageSerialNumber"] == nil{
			continue
		}
		serialNum := item.(map[string]interface{})["storageSerialNumber"].(string)
		hostName := item.(map[string]interface{})["hostName"].(string)
		instName := item.(map[string]interface{})["instanceName"].(string)
		res_data[serialNum] = TStorageApi{instName, hostName}
	}
	return res_data, nil
}

func worker(log *logrus.Logger, api config.TApiTuningManager, storagesApi map[string]TStorageApi, storages []config.TStorage, resources []config.TResource, indexes <-chan [2]int, result chan<- bool, last_run *[][]int64){
	for item:=range(indexes){
		value, err := getData(log, api, storagesApi[storages[item[0]].Serial_Num], storages[item[0]], resources[item[1]])
		if err == nil{
			(*last_run)[item[0]][item[1]] = value
		}
		result <- true
	}
}

func GetAllData (log *logrus.Logger, api config.TApiTuningManager, storagesApi map[string]TStorageApi, storages []config.TStorage, resources []config.TResource, last_run *[][]int64){
	size_queue := len(storages)*len(resources)
	indexes := make(chan [2]int, size_queue)
	result := make(chan bool, size_queue)
	for w := 1; w <= 8; w++ {
		go worker(log, api, storagesApi, storages, resources, indexes, result, last_run)
	}
	count_queue := 0
	for i:= 0; i < len(storages); i++{
		for j := 0; j < len(resources); j++{
			now := time.Now().Unix()
			if (now - (*last_run)[i][j]) > (resources[j].Interval + 5){
				arr_index := [2]int{i, j}
				indexes <- arr_index
				count_queue ++
			}
		}
	}
	close(indexes)
	for i:= 0; i < count_queue; i++{
		<-result
	}
}

func getData(log *logrus.Logger, api config.TApiTuningManager, storageApi TStorageApi, storage config.TStorage, resource config.TResource) (int64, error){
	var result []string
	var last int64
	data, err := getResource(log, api, storageApi, resource.Name)
	if err!=nil{
		log.Warning("Failed to get data ", resource.Name, " from api (", storage.Name, "); Error: ", err)
		return 0, err
	}

	headers := make(map[string]TInfoColumn)
	count := len(data[0])
	for i:=0; i<count; i++{
		headers[data[0][i]] = TInfoColumn{i, data[1][i]}
	}
	if resource.Branch=="capacity"{
		ldevs := make(map[string]map[string]string)
		pools := make(map[string]map[string]string)
		if resource.Type == "LDEV"{
		ldevs, err = getLdevs(log, api, storageApi)
		if err!=nil{
				log.Warning("Failed to get LDEV from ", storage.Name, "; Error: ", err)
				return 0, err
			}
		}
		if resource.Type == "POOL"{
			pools, err = getPools(log, api, storageApi)
			if err!=nil{
				log.Debug("Failed to get POOL ", storage.Name, "; Error: ", err)
				return 0, err
			}
		}
		labels := strings.Split(resource.Label, ",")
		for i:=2; i<len(data); i++{
			labelcontent := "."
			if resource.Label!=""{
				for j:=0; j<len(labels); j++{
					_, flag := headers[labels[j]]
					if flag {
						labelcontent += data[i][headers[labels[j]].index] + "."
					} else {
						labelcontent += labels[j] + "."
					}
				}
			}
			for j:=0; j<len(data[0]); j++{
				if strings.Contains(data[0][j],"CAPACITY")==false{
					continue
				}
				if value_float, err := strconv.ParseFloat(data[i][j],64); err==nil{
					graphitemetric := ""
					value := strconv.FormatFloat(value_float, 'f', 6, 64)
					datetime, _ := time.Parse("2006-01-02 15:04:05", data[i][headers["DATETIME"].index])
					graphitetime:= strconv.FormatInt(datetime.Unix(), 10)
					importmetric := "REALTIME_" + data[0][j]
					if resource.Type == "LDEV"{
						ldev_id := data[i][headers["LDEV_NUMBER"].index]
						ldev_name := ldevs[ldev_id]["ldev_name"]
						pool_id := ldevs[ldev_id]["pool_id"]
						pool_name := ldevs[ldev_id]["pool_name"]
						if pool_id!=""{
							path:= strings.Replace("hds.capacity.physical." + storage.Type + "." + storage.Name + ".LDEV." + pool_id + "." + pool_name + "." + ldev_id + "." + ldev_name + "." + resource.Target + labelcontent + importmetric, " ", "_", -1)
							graphitemetric = path + " " + value + " " + graphitetime
							result = append(result, graphitemetric)
						}
					} else if resource.Type == "POOL"{
						pool_id := data[i][headers["POOL_ID"].index]
						pool_name := pools[pool_id]["pool_name"]
						path := strings.Replace("hds.capacity.physical." + storage.Type + "." + storage.Name + ".POOL." + pool_id + "." + pool_name + "." + resource.Target + labelcontent + importmetric, " ", "_", -1)
						graphitemetric = path + " " + value + " " + graphitetime
						result = append(result, graphitemetric)
					}
				}
			}
		}
	} else {
		ldevs := make(map[string]map[string]string)
		pools := make(map[string]map[string]string)
		if resource.Name == "RAID_PI_LDS"{
			ldevs, err = getLdevs(log, api, storageApi)
			if err!=nil{
				log.Debug("Failed to get LDEV from ", storage.Name, "; Error: ", err)
				return 0, err
			}
		} else if resource.Name == "RAID_PI_PLS"{
			pools, err = getPools(log, api, storageApi)
			if err!=nil{
				log.Debug("Failed to get POOL from ", storage.Name, "; Error: ", err)
				return 0, err
			}
		}
		labels := strings.Split(resource.Label, ",")
		for i:=2; i<len(data); i++{
			labelcontent := "."
			for j:=0; j<len(labels); j++{
				_, flag := headers[labels[j]]
				if flag {
					labelcontent += data[i][headers[labels[j]].index] + "."
				} else {
					labelcontent += labels[j] + "."
				}
			}
			for j:=0; j<len(data[0]); j++{
				if (strings.Contains(headers[data[0][j]].dataType,"string")==false)&&(strings.Contains(headers[data[0][j]].dataType,"time")==false)&&(data[0][j]!="GMT_ADJUST")&&(data[0][j]!="INTERVAL"){
					value_float, _ := strconv.ParseFloat(data[i][j],64)
					value := strconv.FormatFloat(value_float, 'f', 6, 64)
					datetime, _ := time.Parse("2006-01-02 15:04:05", data[i][headers["DATETIME"].index])
					graphitetime:= strconv.FormatInt(datetime.Unix(), 10)
					importmetric := "REALTIME_" + data[0][j]
					graphitemetric := ""
					if resource.Name == "RAID_PI_LDS"{
						ldev_id := labelcontent[1:len(labelcontent)-1]
						parity_grp := ldevs[ldev_id]["parity_grp"]
						pool_id := ldevs[ldev_id]["pool_id"]
						pool_name := ldevs[ldev_id]["pool_name"]
						ldev_name := ldevs[ldev_id]["ldev_name"]
						mp_id := ldevs[ldev_id]["mp_id"]
						if parity_grp != ""{
							graphitemetric = "hds.perf.physical." + storage.Type + "." + storage.Name + "." +  resource.Target + ".PG." + parity_grp + "." + pool_id + "." + pool_name + labelcontent + ldev_name + "." + importmetric + " " + value + " " + graphitetime
							result = append(result, graphitemetric)
						} else {
							if pool_id != ""{
								graphitemetric = "hds.perf.physical." + storage.Type + "." + storage.Name + "." +  resource.Target + ".DP." + pool_id + "." + pool_name + labelcontent + ldev_name + "." + importmetric + " " + value + " " + graphitetime
								result = append(result, graphitemetric)
								graphitemetric = "hds.perf.physical." + storage.Type + "." + storage.Name + ".PRCS." + mp_id + ".LDEV" + labelcontent + importmetric + " " + value + " " + graphitetime
								result = append(result, graphitemetric)
							}
						}
					} else if resource.Name == "RAID_PI_PLS"{
						pool_id := labelcontent[1:len(labelcontent)-1]
						pool_name := pools[pool_id]["pool_name"]
						graphitemetric = "hds.perf.physical." + storage.Type + "." + storage.Name + "." +  resource.Target + labelcontent + pool_name + "." + importmetric + " " + value + " " + graphitetime
						result = append(result, graphitemetric)
					} else {
						graphitemetric = "hds.perf.physical." + storage.Type + "." + storage.Name + "." +  resource.Target + labelcontent + importmetric + " " + value + " " + graphitetime
						result = append(result, graphitemetric)
					}
				}
			}
		}
	}
	if len(result)!=0{
		if err:=sendData.SendObjects(log, result); err!=nil{
			log.Warning("Failed to send ", resource.Name," storage: ", storage.Name, " :Error: ", err)
			return 0, err
		}
		datetime, _ := time.Parse("2006-01-02 15:04:05", data[2][headers["DATETIME"].index])
		last = datetime.Unix()
	}
	return last, nil
}

/*func getDataBypassError(log *logrus.Logger, url string, user string, password string)(data_byte []byte, err error){
	var code int
	for i:=0; i<num_attempts; i++{
		data_byte, code, err = getDataFromApi(log, url, user, password)
		if err!=nil{
			if (code == 503)||(code == 500){
				time.Sleep(time.Second * time.Duration(period_attempts))
				continue
			} else {
				log.Warning("Failed to get data from api (", url, "); Error: ", err)
				return data_byte, err
			}
		} else {
			return data_byte, nil
		}
	}
	log.Warning("The number of connection attempts (", num_attempts, ") has expired (", url, "): Error: ", err)
	return nil, err
}*/

func getDataFromApi(log *logrus.Logger, url string, user string, password string)([]byte, error){
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Warning("Failed to create http request: Error: ", err)
		return nil, err
	}

	req.SetBasicAuth(user, password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warning("Failed to do client request: Error: ", err)
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err!=nil{
		log.Warning("Failed to read response body: Error: ", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return body, nil
	}
	return nil, readErrorByContent(body, resp.Header.Get("Content-Type"), resp.StatusCode)
}

func getResource(log *logrus.Logger, api config.TApiTuningManager, storageApi TStorageApi, resource string)([][]string, error){
	url := api.Protocol + "://" + api.Host + ":" + api.Port + "/TuningManager/v1/objects/" + resource + "?hostName=" + storageApi.HostName + "%26agentInstanceName=" + storageApi.InstanceName
	data_byte, err := getDataFromApi(log, url, api.User, api.Password)
	if err!=nil{
		log.Debug("Failed to get data from api: Error: ", err)
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(data_byte))
	reader.Comma = ','
	res_data, err := reader.ReadAll()
	if err != nil{
		log.Debug("Failed to read byte data: Error: ", err)
		return nil, err
	}

	if len(res_data)<=2{
		err = errors.New("No data in the table " + resource + " from " + storageApi.InstanceName)
		log.Debug(err)
		return nil, err
	}
	return res_data, nil
}

func readErrorByContent(data_byte []byte, content string, code int)(err error){
	switch content{
		case "application/json;charset=utf-8":
			var target interface{}
			json.NewDecoder(bytes.NewReader(data_byte)).Decode(&target)
			err = errors.New(strconv.Itoa(code) + ": " + target.(map[string]interface{})["message"].(string))

		case "text/html;charset=utf-8":
			r := regexp.MustCompile(`<title>(.+)?<\/title>`)
			res := r.FindStringSubmatch(string(data_byte))[1]
			err = errors.New(strconv.Itoa(code) + ": " + res)
	}
	return err
}

func getLdevs (log *logrus.Logger, api config.TApiTuningManager, storageApi TStorageApi)(map[string]map[string]string, error){
	ldevs := make(map[string]map[string]string)
	data, err := getResource(log, api, storageApi, "RAID_PD_LDC")
	if err!=nil{
		log.Debug("Failed to get data from api: Error: ", err)
		return ldevs, err
	}

	pools, err := getPools(log, api, storageApi)
	if err!=nil{
		log.Debug("Failed to get POOL; Error: ", err)
		return ldevs, err
	}
	headers := make(map[string]TInfoColumn)
	for i:=0; i<len(data[0]); i++{
		headers[data[0][i]] = TInfoColumn{i, data[1][i]}
	}

	for i:=2; i<len(data); i++{
		ldev_id := data[i][headers["LDEV_NUMBER"].index]
		ldevs[ldev_id] = make(map[string]string)
		//ldevs[ldev_id]["ldev_name"] = "-"
		//if data[i][headers["LDEV_NAME"].index]!=""{
		//	ldevs[ldev_id]["ldev_name"] = data[i][headers["LDEV_NAME"].index]
		//}
		ldevs[ldev_id]["ldev_name"] = data[i][headers["LDEV_NAME"].index]
		ldevs[ldev_id]["parity_grp"] = data[i][headers["RAID_GROUP_NUMBER"].index]
		ldevs[ldev_id]["pool_id"] = data[i][headers["POOL_ID"].index]
		//ldevs[ldev_id]["pool_name"] = "-"
		//if pools[data[i][headers["POOL_ID"].index]]["pool_name"]!=""{
		//	ldevs[ldev_id]["pool_name"] = pools[data[i][headers["POOL_ID"].index]]["pool_name"]
		//}
		ldevs[ldev_id]["pool_name"] = pools[data[i][headers["POOL_ID"].index]]["pool_name"]
		ldevs[ldev_id]["mp_id"] = data[i][headers["MP_BLADE"].index]
		ldevs[ldev_id]["vldev_id"] = data[i][headers["VIRTUAL_LDEV_NUMBER"].index]
		ldevs[ldev_id]["v_sn"] = data[i][headers["VIRTUAL_SERIAL_NUMBER"].index]
	}
	return ldevs, nil
}

func getPools (log *logrus.Logger, api config.TApiTuningManager, storageApi TStorageApi)(map[string]map[string]string, error){
	pools := make(map[string]map[string]string)
	data, err := getResource(log, api, storageApi, "RAID_PD_PLC")
	if err!=nil{
		log.Debug("Failed to get data from api: Error: ", err)
		return pools, err
	}

	headers := make(map[string]TInfoColumn)
	for i:=0; i<len(data[0]); i++{
		headers[data[0][i]] = TInfoColumn{i, data[1][i]}
	}

	for i:=2; i<len(data); i++{
		pool_id := data[i][headers["POOL_ID"].index]
		pools[pool_id] = make(map[string]string)
		//pools[pool_id]["pool_name"] = "-"
		//if data[i][headers["POOL_NAME"].index] != ""{
		//	pools[pool_id]["pool_name"] = data[i][headers["POOL_NAME"].index]
		//}
		pools[pool_id]["pool_name"] = data[i][headers["POOL_NAME"].index]
	}
	return pools, nil
}
