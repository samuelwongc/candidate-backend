package main

import (
	"bytes"
    "encoding/json"
	"fmt"
	"io"
    "io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/handlers"
    "github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

type Candidate struct {
	gorm.Model
    Firstname   string   	 	  `json:"firstname,omitempty"`
	Lastname    string   		  `json:"lastname,omitempty"`
	Position    string   		  `json:"position,omitempty"`
    Email       string   		  `json:"email,omitempty"`
	Phone       string   		  `json:"phone,omitempty"`
	Stages      []Stage           `json:"stages"`
}

type Stage struct {
	gorm.Model
	Status       string `json:"status,omitempty" sql:"DEFAULT:'pending'"`
	Notes        string `json:"notes,omitempty"`
	Lead         string `json:"lead,omitempty"`
	Datetime     string `json:"datetime,omitempty"`
	Type         string `json:"type,omitempty"`
	CandidateID  uint
}

type Interviewer struct {
	Firstname string
	Lastname  string
	Email     string
}

func NewCandidate(w http.ResponseWriter, r *http.Request) {
	db := getDB()
    decoder := json.NewDecoder(r.Body)
	var newCandidate Candidate
    err := decoder.Decode(&newCandidate)
    if err != nil {
        panic(err)
    }
	db.Create(&newCandidate)
	defer r.Body.Close()
}

func ListCandidate(w http.ResponseWriter, r *http.Request) {
	db := getDB()
	var rawCandidates []Candidate
	db.Find(&rawCandidates)

	var candidates []Candidate

	for _, candidate := range rawCandidates {
		var stages []Stage
		db.Model(&candidate).Related(&stages)
		candidate.Stages = stages
		candidates = append(candidates, candidate)
	}

	if err := json.NewEncoder(w).Encode(candidates); err != nil {
        panic(err)
    }
}

func GetCandidate(w http.ResponseWriter, r *http.Request) {
	db := getDB()
	vars := mux.Vars(r)
	candidateId, err := strconv.Atoi(vars["candidateId"])
	if err != nil {
		panic(err)
	}
	var candidate Candidate
	db.First(&candidate, candidateId)
	var stages []Stage
	db.Model(&candidate).Related(&stages)
	candidate.Stages = stages
	if err := json.NewEncoder(w).Encode(candidate); err != nil {
        panic(err)
    }
}

func DownloadCandidateCv(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	streamPDFbytes, err := ioutil.ReadFile(fmt.Sprintf("/tmp/%s-cv.pdf", vars["candidateId"]))

	if err != nil {
			fmt.Println(err)
			os.Exit(1)
	}

	b := bytes.NewBuffer(streamPDFbytes)

	w.Header().Set("Content-type", "application/pdf")

	if _, err := b.WriteTo(w); err != nil { // <----- here!
		fmt.Fprintf(w, "%s", err)
	}
}


func UploadCandidateCv(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

    var buffer bytes.Buffer
    // in your case file would be fileupload
    file, _, err := r.FormFile("file")
    if err != nil {
        panic(err)
    }
	
	io.Copy(&buffer, file)

	log.Println(fmt.Sprintf("/tmp/%s-cv.pdf", vars["candidateId"]))
	ioutil.WriteFile(fmt.Sprintf("/tmp/%s-cv.pdf", vars["candidateId"]), buffer.Bytes(), 0644)
    
    buffer.Reset()
}

func EditStage(w http.ResponseWriter, r *http.Request) {
	db := getDB()

	decoder := json.NewDecoder(r.Body)

	var stage Stage
    err := decoder.Decode(&stage)
    if err != nil {
        panic(err)
	}
	
	vars := mux.Vars(r)
	var currStage Stage
	db.First(&currStage, vars["stageId"])
	db.Model(&currStage).Updates(stage)
}

func PassStage(w http.ResponseWriter, r *http.Request) {
	db := getDB()

	vars := mux.Vars(r)
	var currStage Stage
	db.First(&currStage, vars["stageId"])
	db.Model(&currStage).Update("status", "pass")
}

func FailStage(w http.ResponseWriter, r *http.Request) {
	db := getDB()

	vars := mux.Vars(r)
	var currStage Stage
	db.First(&currStage, vars["stageId"])
	db.Model(&currStage).Update("status", "fail")
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "Hello world!\n")
}

func getDB() *gorm.DB {
	dbhost := os.Getenv("DB_HOST")
	dbport := os.Getenv("DB_PORT")
	dbname := os.Getenv("DB_NAME")
	dbuser := os.Getenv("DB_USER")
	dbpassword := os.Getenv("DB_PASSWORD")

	db, err := gorm.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s", dbhost, dbport, dbuser, dbname, dbpassword))
	if err != nil {
		panic(err)
	}
	return db
}

func main() {
	db := getDB()

	log.Println("Running migrations...")
	db.AutoMigrate(&Candidate{})
	db.AutoMigrate(&Stage{})

	log.Println("Setting up router...")

	corsObj:=handlers.AllowedOrigins([]string{"*"})
    r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/candidate/", ListCandidate).Methods("GET")
	r.HandleFunc("/candidate/", NewCandidate).Methods("POST")
	r.HandleFunc("/candidate/{candidateId}/", GetCandidate).Methods("GET")
	r.HandleFunc("/candidate/{candidateId}/cv/", UploadCandidateCv).Methods("POST")
	r.HandleFunc("/candidate/{candidateId}/stages/{stageId}/", EditStage).Methods("POST")
	r.HandleFunc("/candidate/{candidateId}/stages/{stageId}/pass/", PassStage).Methods("POST")
	r.HandleFunc("/candidate/{candidateId}/stages/{stageId}/fail/", FailStage).Methods("POST")
	r.HandleFunc("/candidate/{candidateId}/cv/", DownloadCandidateCv).Methods("GET")

	port := 4000
	log.Println(fmt.Sprintf("Listening to port %d", port))
	srv := &http.Server{
		Handler:      handlers.CORS(corsObj)(r),
		Addr:         fmt.Sprintf(":%d", port),
		WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }

    log.Fatal(srv.ListenAndServe())
}