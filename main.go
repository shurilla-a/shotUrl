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
	"runtime"
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
// константы для подключения к монгоДБ
//const (
//	DBName        = " testTest"
//	URI           = "mongodb://127.0.0.1:27017"
//	UrlCollection = "shotUrl"
//)

// описание структуры для вставки в монгоДБ
//type ShotUrl struct {
//	ID    primitive.ObjectID `bson:"id" json:"id,omitempty"`
//	Key   string             `json:"key"`
//	Value string             `json:"value"`
//}

//Функция подключения к монгоДБ
//func mongoDbC() (*mongo.Client, *mongo.Database) {
//	ctx := context.Background()
//	mDBcop := options.Client().ApplyURI(URI)
//	mDBcon, err := mongo.Connect(ctx, mDBcop)
//	if err != nil {
//		log.Println("Функция mongoDbC не возможно подключиться к mongoDB", err)
//	}
//	mDBnameDB := mDBcon.Database(DBName)
//	return mDBcon, mDBnameDB
//}

// проверка доступности MongoDB
//func CheckMongoDB(mDBcon *mongo.Client) bool {
//	err := mDBcon.Ping(context.TODO(), nil)
//	if err != nil {
//		log.Println("Функция CheckMongoDB , mongoDB не доступна", err)
//		return false
//	} else {
//		log.Println("Функция CheckMongoDB , mongoDB доступна")
//		return true
//	}
//}

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
		log.Println("Функция RedisConnect Не удалось подключиться к REDIS ", check)
	} else {
		log.Println("Функция RedisConnect Соединение с REDIS установлено ", check)
	}
	return rdbc
}

// проверка доступности REDIS
func CheckRedisConnect(rdbc *redis.Client) bool {
	pong, err := rdbc.Ping().Result()
	if err != nil {
		log.Println("Функция CheckRedisConnect не удалось подключиться к REDIS ", err)
		return false
	} else {
		log.Println("Функция CheckRedisConnect соединение с REDIS установлено ", pong)
		return true
	}
}

// Функция Генерации Ключей для связки ключ:значние
//func GenerateKey(rdbc *redis.Client, mDBcon *mongo.Client, mDBnameDB *mongo.Database) (string, bool)
func GenerateKey(rdbc *redis.Client) (string, bool) {
	check := CheckRedisConnect(rdbc)
	if check != true {
		log.Println("Функция GenerateKey , Redis не доступен", check)
		return "", false
	}

	//checkMongo := CheckMongoDB(mDBcon)
	//if checkMongo != true {
	//	log.Println("Функция GenerateKey , MongoDB  не доступена", checkMongo)
	//	return "", false
	//}

	hd := hashids.NewData()
	hd.MinLength = 7
	hash, err := hashids.NewWithData(hd)
	if err != nil {
		log.Println("Функция GenerateKey не возможно создать New new HashID ", err)
		return "", false
	}
	timeNow := time.Now()
	key, err := hash.Encode([]int{int(timeNow.Nanosecond())})
	if err != nil {
		log.Println("Функция GenerateKey не возможно Encode hashes ", err)
		return "", false
	}
	value, err := rdbc.Get(key).Result()
	if err == redis.Nil {
		log.Println("Функция GenerateKey Значение по ключу "+key+" не найдено", err)
	} else {
		log.Println("Функция GenerateKey Ключ " + key + " со значением " + value + " существует ")
		//GenerateKey(rdbc,mDBcon,mDBnameDB)
		GenerateKey(rdbc)
	}

	return key, true
}

// Функция Редирект с короткой ссылки на обычную
func Redirect(w http.ResponseWriter, req *http.Request, rdbc *redis.Client) {
	params := mux.Vars(req)
	key := params["key"]
	url, err := rdbc.Get(key).Result()
	if err != nil {
		log.Println("Функция Redirect НЕ утдалось перенаправить по ключу "+key+" Ошибка", err)
		check := CheckRedisConnect(rdbc)
		if check != true {
			ReturnCode500(w)
			return
		}
	} else {
		http.Redirect(w, req, url, 301)
	}

}

//Функция создания короткой ссылки
func Create(w http.ResponseWriter, req *http.Request, rdbc *redis.Client) {
	check := CheckRedisConnect(rdbc)
	if check != true {
		log.Println("Функция Create,Redis не доступен", check)
		ReturnCode500(w)
		return
	}
	req.ParseForm()
	url := req.Form["url"][0]
	key, genkeyBool := GenerateKey(rdbc)
	if genkeyBool != true {
		log.Println("Ошибка при работе функции GenerateKey ", genkeyBool)
		ReturnCode500(w)
		return
	}
	value, err := rdbc.Get(key).Result()
	if err == redis.Nil {
		_, err := rdbc.Set(key, url, 0).Result()
		log.Println("Значение по ключу "+key+" Сохранено", err)
		fmt.Fprintln(w, "http://o.cmrka.df/"+key)
	} else {
		log.Println("НЕ возможно записать ключ " + key + " ошибка Значение " + value + "Существет")
		ReturnCode500(w)
		return
	}
}

//Функция Error 500
func ReturnCode500(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 - Something bad happened!"))
}

func main() {
	runtime.GOMAXPROCS(2)
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
	http.ListenAndServe(":8080", router)
	//if err != nil {
	//	log.Fatal(http.ListenAndServe(":3128",nil))
	//}
	//
}
