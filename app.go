package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/gorilla/mux"
)

type App struct {
	Router *mux.Router
	DB     *sql.DB
}

func (a *App) Initialize(user string, password string, dbName string, dbConnection string) {
	fmt.Println("Start to Initialize App")

	var err error

	connectionString := user + ":" + password + "@/" + dbName

	a.DB, err = sql.Open(dbConnection, connectionString)
	if err != nil {
		log.Fatalf("[sql.Open] %s", err.Error())
	}

	driver, err := mysql.WithInstance(a.DB, &mysql.Config{})
	if err != nil {
		log.Fatalf("[mysql.WithInstance] %s", err.Error())
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://./database/migrations",
		dbName,
		driver,
	)
	if err != nil {
		log.Fatalf("[NewWithDatabaseInstance] %s", err.Error())
	}

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("[Up] %s", err.Error())
	}

	a.Router = mux.NewRouter()

	a.initializeRoutes()
}

func (a *App) Run(port string) {
	fmt.Println("App is running on port", port)

	if err := http.ListenAndServe(":"+port, a.Router); err != nil {
		log.Fatalf("[ListendAndServe] %s", err.Error())
	}
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/carts", a.getCarts).Methods(http.MethodGet)
	a.Router.HandleFunc("/cart/{cartID}", a.getCartByID).Methods(http.MethodGet)
	a.Router.HandleFunc("/cart", a.createCart).Methods(http.MethodPost)
	a.Router.HandleFunc("/cart/{cartID}", a.addItemToCart).Methods(http.MethodPost)
	a.Router.HandleFunc("/cart/{cartID}", a.deleteCart).Methods(http.MethodDelete)
	a.Router.HandleFunc("/cart/{cartID}/item/{itemID}", a.deleteCartItem).Methods(http.MethodDelete)
}

func (a *App) getCarts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.FormValue("limit"))
	offset, _ := strconv.Atoi(r.FormValue("offset"))

	if limit <= 0 {
		limit = 10
	}

	carts, err := getCarts(a.DB, limit, offset)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			respondWithError(w, http.StatusNotFound, "Cart not found")
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if len(carts) == 0 {
		respondWithError(w, http.StatusNotFound, "Cart not found")
		return
	}

	var respHTTP = struct {
		Carts []cart `json:"carts"`
	}{
		Carts: carts,
	}

	respondWithJSON(w, http.StatusOK, respHTTP)
}

func (a *App) getCartByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	c := cart{ID: vars["cartID"]}

	if err := c.getCartByID(a.DB); err != nil {
		switch err {
		case sql.ErrNoRows:
			respondWithError(w, http.StatusNotFound, "Cart not found")
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	var respHTTP = struct {
		Cart cart `json:"cart"`
	}{
		Cart: c,
	}

	respondWithJSON(w, http.StatusOK, respHTTP)
}

func (a *App) createCart(w http.ResponseWriter, r *http.Request) {
	var c cart

	if err := c.createCart(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, c)
}

func (a *App) addItemToCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	var ci cartItem

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&ci); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	ci.CartID = vars["cartID"]

	c, err := ci.addItemToCart(a.DB)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, c)
}

func (a *App) deleteCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	c := cart{ID: vars["cartID"]}
	if err := c.deleteCart(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) deleteCartItem(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	ci := cartItem{
		ID:     vars["itemID"],
		CartID: vars["cartID"],
	}

	if err := ci.deleteCartItem(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	fmt.Println(message)
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
