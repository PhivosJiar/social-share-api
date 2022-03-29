package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/bachvtuan/shortmongoid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type previewInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageUrl    string `json:"imageUrl"`
	TargetUrl   string `json:"targetUrl"`
	ShareUrlId  string `json:"shareUrlId"`
}

type Id struct {
	ObjectId string `json:"objectId"`
}

func main() {

	port := "8082"
	if v := os.Getenv("PORT"); len(v) > 0 {
		port = v
	}

	// 使用 int 型 uid 生成 RTC Token
	http.HandleFunc("/generator_url", savePreviewInfo)
	http.HandleFunc("/find_preview_info", findPreviewInfo)
	fmt.Printf("Starting server at port 8082\n")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}

	fmt.Println("==========================")
}

func findPreviewInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" && r.Method != "OPTIONS" {
		http.Error(w, "Unsupported method. Please check.", http.StatusNotFound)
		return
	}

	var Id Id
	var unmarshalErr *json.UnmarshalTypeError
	req_decoder := json.NewDecoder(r.Body)
	req_decoder_err := req_decoder.Decode(&Id)

	if req_decoder_err == nil {
		id := Id.ObjectId
		findPreviewInfoById(id, w)
	}

	if req_decoder_err != nil {
		if errors.As(req_decoder_err, &unmarshalErr) {
			errorResponse(w, "Bad request.  Wrong type provided for field "+unmarshalErr.Value+unmarshalErr.Field+unmarshalErr.Struct, http.StatusBadRequest)
		} else {
			errorResponse(w, "Bad request.", http.StatusBadRequest)
		}
		return
	}
}

func findPreviewInfoById(id string, w http.ResponseWriter) {
	clientOptions := options.Client().ApplyURI("mongodb://127.0.0.1:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if client != nil {
		fmt.Println("connet mongo success")
	}
	if err != nil {
		errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var previewInfo previewInfo
	c := client.Database("socialShare").Collection("previewInfo")
	filter := bson.M{"shareurlid": id}
	if err = c.FindOne(context.TODO(), filter).Decode(&previewInfo); err != nil {
		errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	findSuccessResponse(w, previewInfo, http.StatusOK)
}

func savePreviewInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" && r.Method != "OPTIONS" {
		http.Error(w, "Unsupported method. Please check.", http.StatusNotFound)
		return
	}

	var previewInfo previewInfo
	var unmarshalErr *json.UnmarshalTypeError
	preview_decoder := json.NewDecoder(r.Body)
	preview_decoder_err := preview_decoder.Decode(&previewInfo)

	if preview_decoder_err == nil {
		title := previewInfo.Title
		description := previewInfo.Description
		imgUrl := previewInfo.ImageUrl
		targetUrl := previewInfo.TargetUrl
		shareUrlId := previewInfo.ShareUrlId
		if targetUrl == "" {
			errorResponse(w, "Bad request. targetUrl is null", http.StatusBadRequest)
			return
		}

		generatorDynamicUrl(title, description, imgUrl, targetUrl, shareUrlId, w)
	}

	if preview_decoder_err != nil {

		if errors.As(preview_decoder_err, &unmarshalErr) {
			errorResponse(w, "Bad request.  Wrong type provided for field "+unmarshalErr.Value+unmarshalErr.Field+unmarshalErr.Struct, http.StatusBadRequest)
		} else {
			errorResponse(w, "Bad request.", http.StatusBadRequest)
		}
		return
	}
}

func generatorDynamicUrl(title string, description string, imageUrl string, targetUrl string, shareUrlId string, w http.ResponseWriter) {
	clientOptions := options.Client().ApplyURI("mongodb://127.0.0.1:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c := client.Database("socialShare").Collection("previewInfo")
	result, err := c.InsertOne(
		context.TODO(),
		NewPreviewInfo(title, description, imageUrl, targetUrl, shareUrlId))

	if err != nil {
		errorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result != nil {
		oid := result.InsertedID.(primitive.ObjectID)
		key, err := shortmongoid.ShortId(oid.Hex())
		if err != nil {
			errorResponse(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hex := oid.Hex()
		id, _ := primitive.ObjectIDFromHex(hex)

		fmt.Println(key)
		result, err := c.UpdateOne(
			context.TODO(),
			bson.M{"_id": id},
			bson.D{
				{Key: "$set", Value: bson.D{{Key: "shareurlid", Value: key}}},
			},
		)

		if err != nil {
			errorResponse(w, err.Error(), http.StatusInternalServerError)
		}
		fmt.Printf("Updated %v Documents!\n", result.ModifiedCount)
		successResponse(w, key, http.StatusOK)
		return
	}
}

func errorResponse(w http.ResponseWriter, message string, httpStatusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(httpStatusCode)
	resp := make(map[string]string)
	resp["token"] = message
	resp["code"] = strconv.Itoa(httpStatusCode)
	jsonResp, _ := json.Marshal(resp)
	w.Write(jsonResp)
}

func successResponse(w http.ResponseWriter, message string, httpStatusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(httpStatusCode)
	resp := make(map[string]string)
	resp["url"] = "https://socialShare.link/" + message
	resp["code"] = strconv.Itoa(httpStatusCode)
	jsonResp, _ := json.Marshal(resp)
	w.Write(jsonResp)
}

func findSuccessResponse(w http.ResponseWriter, previewInfo previewInfo, httpStatusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(httpStatusCode)
	resp := make(map[string]string)
	resp["title"] = previewInfo.Title
	resp["description"] = previewInfo.Description
	resp["imageUrl"] = previewInfo.ImageUrl
	resp["targetUrl"] = previewInfo.TargetUrl
	resp["code"] = strconv.Itoa(httpStatusCode)
	jsonResp, _ := json.Marshal(resp)
	w.Write(jsonResp)
}

func NewPreviewInfo(title string, description string, imageUrl string, targetUrl string, shareUrlId string) *previewInfo {

	return &previewInfo{
		Title:       title,
		Description: description,
		ImageUrl:    imageUrl,
		TargetUrl:   targetUrl,
		ShareUrlId:  shareUrlId,
	}
}
