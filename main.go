package main

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/speps/go-hashids"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

var (
	ErrorLogger *log.Logger
	InfoLogger  *log.Logger
)

func init() {
	logfile, err := os.OpenFile("nweLog.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	InfoLogger = log.New(logfile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(logfile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

// Функция подключения к БД REDIS
func RedisConnect() *redis.Client {
	rdbc := redis.NewClient(&redis.Options{
		DB:         0,
		Addr:       "127.0.0.1:6379",
		Password:   "",
		MaxRetries: 5,
	})
	check := CheckRedisConnect(rdbc)
	if check != true {
		ErrorLogger.Println("Функция RedisConnect Не удалось подключиться к REDIS ", check)
	} else {
		InfoLogger.Println("Функция RedisConnect Соединение с REDIS установлено ", check)
	}
	return rdbc
}

// проверка доступности REDIS
func CheckRedisConnect(rdbc *redis.Client) bool {
	pong, err := rdbc.Ping().Result()
	if err != nil {
		ErrorLogger.Println("Функция CheckRedisConnect не удалось подключиться к REDIS ", err)
		return false
	} else {
		InfoLogger.Println("Функция CheckRedisConnect соединение с REDIS установлено " + pong)
		return true
	}
}

// Функция Генерации Ключей для связки ключ:значние
func GenerateHash() (error, string) {
	hd := hashids.NewData()
	hd.MinLength = 0
	hash, err := hashids.NewWithData(hd)
	if err != nil {
		ErrorLogger.Println("Функция GenerateHash не возможно создать New new HashID ", err)
		//LogError(err, "Функция GenerateHash не возможно создать New new HashID ")
		return err, ""
	}
	timeNow := time.Now()
	key, err := hash.Encode([]int{int(timeNow.Nanosecond())})
	if err != nil {
		ErrorLogger.Println("Функция GenerateHash не возможно Encode hashes ", err)
		return err, ""
	} else {
		InfoLogger.Println("Ключ " + key + "Сгенерирован фунуцией GenerateHash")
		return nil, key
	}
}

func GenerateKey(rdbc *redis.Client) (error, string) {
	err, key := GenerateHash()
	if err != nil {
		return err, ""
	}
	InfoLogger.Println("Ключ сгенерирован ", key)
	value, err := rdbc.Get(key).Result()
	if err == redis.Nil {
		InfoLogger.Println("Функция GenerateKey Значение по ключу "+key+" не найдено ", err)
	} else {
		ErrorLogger.Println("Функция GenerateKey Ключ "+key+" со значением "+value+" существует ", err)
		_, key = GenerateKey(rdbc)
	}
	return nil, key
}

// Функция Редирект с короткой ссылки на обычную
func Redirect(w http.ResponseWriter, req *http.Request, rdbc *redis.Client) {
	params := mux.Vars(req)
	key := params["key"]
	url, err := rdbc.Get(key).Result()
	if err != nil {
		ErrorLogger.Println("Функция Redirect НЕ утдалось перенаправить по ключу "+key+"ошибка ", err)
		ReturnCode404(w)
		return
	} else {
		http.Redirect(w, req, url, 301)
	}
}

//Функция создания короткой ссылки
func Create(w http.ResponseWriter, req *http.Request, rdbc *redis.Client) {
	check := CheckRedisConnect(rdbc)
	if check != true {
		ErrorLogger.Println("Функция Create,Redis не доступен ", check)
		ReturnCode500(w)
		return
	}
	req.ParseForm()
	url := req.Form["url"][0]
	err, key := GenerateKey(rdbc)
	if err != nil {
		ReturnCode500(w)
		return
	}
	value, err := rdbc.Get(key).Result()
	if err == redis.Nil {
		InfoLogger.Println("Значение по ключу " + key + " не найдено ")
		value, err = rdbc.Set(key, url, 0).Result()
		if err != nil {
			ErrorLogger.Println("При записи ключа "+key+" редис возникла ошибка ", err)
			ReturnCode500(w)
			return
		}
		InfoLogger.Println("Значение по ключу " + key + " Сохранено")
		fmt.Fprintln(w, "http://o.XXXX.df/"+key)
	} else {
		ErrorLogger.Println("НЕ возможно записать ключ "+key+" ошибка Значение "+value+" Существет ", err)
		ReturnCode500(w)
		return
	}
}

//Функция Error 500
func ReturnCode500(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 - Something bad happened!"))
}

func ReturnCode404(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 - Page not found "))
}

func main() {
	runtime.GOMAXPROCS(2)
	rdbc := RedisConnect()
	signalChanel := make(chan os.Signal, 1)
	signal.Notify(signalChanel, syscall.SIGQUIT)
	go func() {
		s := <-signalChanel
		InfoLogger.Printf("рограмма завершена по сигналу %s", s)
		os.Exit(1)
	}()
	router := mux.NewRouter()
	router.HandleFunc("/{key}", func(w http.ResponseWriter, req *http.Request) {
		Redirect(w, req, rdbc)
	}).Methods("GET")
	router.HandleFunc("/create", func(w http.ResponseWriter, req *http.Request) {
		Create(w, req, rdbc)
	}).Methods("POST")
	http.ListenAndServe(":8080", router)
}
