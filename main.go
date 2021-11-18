package main

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/speps/go-hashids"
	_ "gopkg.in/yaml.v2"
	"log"
	"net/http"
	"os"
	"time"
)

//структура файла конфигурации
//type ConfigYaml struct {
//	Host       string `yaml:"host"`
//	Port       string `yaml:"port"`
//	Password   string `yaml:"passwd"`
//	MaxRetries int    `yaml:"maxretries"`
//	DB         int    `yaml:"db"`
//	KeyLength  int    `yaml:"keylength"`
//	TtlKey     int    `yaml:"ttl-key"`
//}
//
//// функция обработки файла конфигурации
//func ConfigFile(configFile string) (*ConfigYaml, error) {
//
//}

// Функция подключения к БД REDIS
func RedisConnect() *redis.Client {
	rdbc := redis.NewClient(&redis.Options{
		DB:         0,
		Addr:       "127.0.0.1:6379",
		Password:   "",
		MaxRetries: 5,
	})
	pong, err := rdbc.Ping().Result()
	if err != nil {
		log.Println("Не удалось подключиться к REDIS ", err)
		time.Sleep(2 * time.Minute)
		RedisConnect()

	} else {
		log.Println("Соединение с REDIS установлено ", pong)
	}
	return rdbc
}
func CheckRedisConnect(rdbc *redis.Client) bool {
	pong, err := rdbc.Ping().Result()
	if err != nil {
		log.Println("Не удалось подключиться к REDIS ", err)
		return false
	} else {
		log.Println("Соединение с REDIS установлено ", pong)
		return true
	}
}

// Функция Генерации Ключей для связки ключ:значние
func GenerateKey(rdbc *redis.Client) string {
	chech := CheckRedisConnect(rdbc)
	if chech != true {
		RedisConnect()
	}
	hd := hashids.NewData()
	hd.MinLength = 7
	hash, err := hashids.NewWithData(hd)
	if err != nil {
		log.Println(err)
	}
	timeNow := time.Now()
	key, err := hash.Encode([]int{int(timeNow.Unix())})
	if err != nil {
		log.Println(err)
	}
	value, err := rdbc.Get(key).Result()
	if err == redis.Nil {
		log.Println("Значение по ключу "+key+" не найдено", err)
	} else if err != nil {
		log.Println("Не удалось проверить ключ или REDIS не доступен", err)
	} else {
		log.Println("Ключ " + key + " со значением " + value + " существует ")
		GenerateKey(rdbc)
	}
	return key

}

// Функция Редирект с короткой ссылки на обычную
func Redirect(w http.ResponseWriter, req *http.Request, rdbc *redis.Client) {
	params := mux.Vars(req)
	key := params["key"]
	url, err := rdbc.Get(key).Result()
	if err != nil {
		log.Println(err)
	}
	http.Redirect(w, req, url, 301)
	//defer rdbc.Close()
}

//Функция создания короткой ссылки
func Create(w http.ResponseWriter, req *http.Request, rdbc *redis.Client) {
	req.ParseForm()
	url := req.Form["url"][0]
	key := GenerateKey(rdbc)
	_, err := rdbc.Set(key, url, 0).Result()
	if err != nil {
		log.Println("НЕ возможно записать ключ "+key+" ошибка ", err)
		//http.Redirect(w,req,"http://127.0.0.1:3128/error",307)
		RedisConnect()
	}
	log.Println("Значение по ключу " + key + " Сохранено")
	fmt.Fprintln(w, "http://127.0.0.1:3128/"+key)
	//defer rdbc.Close()
}

//Функция Error 500
func ReturnCode500(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("HTTP status code returned!"))
}
func main() {
	logFile, err := os.OpenFile("work.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Panicf("Не возможно создать или открыть лог ошибок", err)
	}
	log.SetOutput(logFile)
	rdbc := RedisConnect()
	router := mux.NewRouter()
	router.HandleFunc("/{key}", func(w http.ResponseWriter, req *http.Request) {
		Redirect(w, req, rdbc)
	}).Methods("GET")
	router.HandleFunc("/create", func(w http.ResponseWriter, req *http.Request) {
		Create(w, req, rdbc)
	}).Methods("POST")
	router.HandleFunc("/error", ReturnCode500)
	http.ListenAndServe(":3128", router)
}
