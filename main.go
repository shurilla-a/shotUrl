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
		LogError(nil, "Функция RedisConnect Не удалось подключиться к REDIS ")

	} else {
		LogInform("Функция RedisConnect Соединение с REDIS установлено ")
	}
	return rdbc
}

// проверка доступности REDIS
func CheckRedisConnect(rdbc *redis.Client) bool {
	pong, err := rdbc.Ping().Result()
	if err != nil {
		LogError(err, "Функция CheckRedisConnect не удалось подключиться к REDIS ")
		return false
	} else {
		LogInform("Функция CheckRedisConnect соединение с REDIS установлено " + pong)
		return true
	}
}

//Функция обработки логов ошибок
func LogError(err error, msg string) {
	logFile, err1 := os.OpenFile("work.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err1 != nil {
		log.Panicf("Не возможно создать или открыть лог ошибок", err)
	}
	//InfoLogger = log.New(logFile, "INFO", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	ErrorLog := log.New(logFile, "ERROR - ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	ErrorLog.Println(err, msg)
	log.SetOutput(logFile)
}

//Функция LogInform
func LogInform(msg string) {
	logFile, err1 := os.OpenFile("work.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err1 != nil {
		log.Panicf("Не возможно создать или открыть лог ошибок", err1)
	}
	ErrorLog := log.New(logFile, "INFO - ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	ErrorLog.Println(msg)
	log.SetOutput(logFile)
}

// Функция Генерации Ключей для связки ключ:значние
func GenerateHash() (error, string) {
	hd := hashids.NewData()
	hd.MinLength = 0
	hash, err := hashids.NewWithData(hd)
	if err != nil {
		LogError(err, "Функция GenerateHash не возможно создать New new HashID ")
		return err, ""
	}
	timeNow := time.Now()
	//fmt.Println(timeNow.Nanosecond())
	key, err := hash.Encode([]int{int(timeNow.Nanosecond())})
	if err != nil {
		LogError(err, "Функция GenerateHash не возможно Encode hashes ")
		return err, ""
	} else {
		LogInform("Ключ " + key + "Сгенерирован фунуцией GenerateHash")
		return nil, key
	}
}

func GenerateKey(rdbc *redis.Client) (error, string) {
	err, key := GenerateHash()
	if err != nil {
		return err, ""
	}
	LogInform("Ключ сгенерирован " + key)
	value, err := rdbc.Get(key).Result()
	if err == redis.Nil {
		LogInform("Функция GenerateKey Значение по ключу " + key + " не найдено")
		//log.Println("Функция GenerateKey Значение по ключу "+key+" не найдено", err)
	} else {
		LogError(err, "Функция GenerateKey Ключ "+key+" со значением "+value+" существует")
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
		LogError(err, "Функция Redirect НЕ утдалось перенаправить по ключу "+key)
		//ErrorLogger.Println("Функция Redirect НЕ утдалось перенаправить по ключу "+key+" Ошибка", err)
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

		LogError(nil, "Функция Create,Redis не доступен")
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
		LogInform("Значение по ключу не найдено" + key)
		value, err = rdbc.Set(key, url, 0).Result()
		if err != nil {
			LogError(err, "Ошибка при записи ключа "+key+" редис")
			ReturnCode500(w)
			return
		}
		LogInform("Значение по ключу " + key + " Сохранено")
		//log.Println("Значение по ключу "+key+" Сохранено", err)
		fmt.Fprintln(w, "http://o.XXXX.df/"+key)
	} else {
		LogError(err, "НЕ возможно записать ключ "+key+" ошибка Значение "+value+" Существет")
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
		log.Printf("рограмма завершена по сигналу %s", s)
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
