package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/cheggaaa/pb/v3"
	"github.com/gocolly/colly"
	"golang.org/x/sync/semaphore"
)

/*DealCount represents Number of deal to be fetched*/
const DealCount = 100000

// CarDeal stores information about a car deal
type CarDeal struct {
	ID              int
	Manufacturer    string
	Model           string
	Year            string
	Category        string
	Mileage         string
	FuelType        string
	EngineVolume    string
	DriveWheels     string
	GearBox         string
	Doors           string
	Wheel           string
	Color           string
	InteriorColor   string
	VIN             string
	LeatherInterior bool
	Price           float32
	Clearance       bool
	ImageURLs       []string
}

func (c *CarDeal) toString() *[]string {
	r := []string{
		strconv.Itoa(c.ID), c.Manufacturer, c.Model, c.Year, c.Category,
		c.Mileage, c.FuelType, c.EngineVolume, c.DriveWheels, c.GearBox, c.Doors, c.Wheel, c.Color,
		c.InteriorColor, c.VIN, strconv.FormatBool(c.LeatherInterior), strconv.Itoa(int(c.Price)),
		strconv.FormatBool(c.Clearance),
	}
	return &r
}

/*NewEmptyCarDeal Return new empty car deal*/
func NewEmptyCarDeal() *CarDeal {
	newDeal := CarDeal{
		ID:              0,
		Manufacturer:    "",
		Model:           "",
		Year:            "",
		Category:        "",
		Mileage:         "",
		FuelType:        "",
		EngineVolume:    "",
		DriveWheels:     "",
		GearBox:         "",
		Doors:           "",
		Wheel:           "",
		Color:           "",
		InteriorColor:   "",
		VIN:             "",
		LeatherInterior: false,
		Price:           0.0,
		Clearance:       false,
		ImageURLs:       []string{},
	}

	return &newDeal
}

func main() {
	//To wait for go routines
	var wg sync.WaitGroup
	//Semaphore for content download, 1000 handler at once maximum
	downSem := semaphore.NewWeighted(1000)
	//Progess bar
	bar := pb.StartNew(DealCount)
	//Get ready to write to data file
	DWMutex := &sync.Mutex{}
	os.MkdirAll("MyAutoData/images/", 0755)
	dataFile, dataWriter := createCSV("MyAutoData/data.csv")
	defer dataFile.Close()
	defer dataWriter.Flush()

	//Create client for image download
	client := &http.Client{}
	//Default collector to go through pages and call detail collector on each entry
	c := colly.NewCollector(
		colly.AllowedDomains("myauto.ge", "www.myauto.ge"),
		colly.CacheDir("./.cache"),
	)

	// Create another collector to scrape deal details
	detailCollector := c.Clone()

	carDeals := make([]CarDeal, DealCount)

	// On Every page under this element is the link to a car deal
	c.OnHTML("figure[class=search-list-figure] > a[href]", func(e *colly.HTMLElement) {
		//Extracting link of entry
		link := e.Attr("href")
		// start scaping the page under the link found
		detailCollector.Visit(link)
	})

	//Each deal is gonna be pushed here
	dealsChannel := make(chan *CarDeal, 100)
	dealExtraction(dealsChannel, detailCollector)
	//Iterating over pages
	go iterateOverPages(c)
	for i := 0; i < DealCount; i++ {
		aDeal := <-dealsChannel
		aDeal.ID = i
		carDeals[i] = *aDeal

		wg.Add(1)
		go downloadImages(i, &aDeal.ImageURLs, client, downSem, &wg)
		bar.Increment()
		if i%100 == 0 && i != 0 {
			// fmt.Printf("Pushing 100 deals, up to %d into the buffer...\n", i)
			wg.Add(1)
			go writeToWriter(dataWriter, &carDeals, i-100, i, DWMutex, &wg)
		}
	}

	bar.Finish()
	wg.Wait()
}

func downloadImages(id int, urls *[]string, client *http.Client, sem *semaphore.Weighted, wg *sync.WaitGroup) {
	defer wg.Done()
	//Accuire lock for urls
	sem.Acquire(context.TODO(), int64(len(*urls)))
	//Release them at the end
	defer sem.Release(int64(len(*urls)))

	dir := "MyAutoData/images/" + strconv.Itoa(id)
	err := os.MkdirAll(dir, 0755) //Create directory
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't create directory for deal images with id: %d\n%v\n", id, err)
		return
	}
	for i, url := range *urls {
		urlTok := strings.Split(url, ".")
		fileExt := urlTok[len(urlTok)-1] //Get file extension
		if strings.Contains(fileExt, "?") {
			fileExt = fileExt[0:strings.Index(fileExt, "?")]
		}

		f, err := os.Create(dir + "/" + strconv.Itoa(i) + "." + fileExt)
		defer f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Couldn't create deal's(id:%d) image %d\n%v\n", id, i, err)
			continue
		}
		resp, err := client.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Couldn't download image for deal %d\n%s\n%v\n", id, url, err)
			continue
		}
		defer resp.Body.Close()
		_, err = io.Copy(f, resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Couldn't write image for deal %d\n%s\n%v\n", id, url, err)
			continue
		}
		f.Close()
	}
}

func writeToWriter(dw *csv.Writer, carDeals *[]CarDeal, from int, to int, lock *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	lock.Lock()
	for i := from; i < to; i++ {
		err := dw.Write(*(*carDeals)[i].toString())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error occured while writing to a file\n%v\n", err)
		}
	}
	dw.Flush()
	lock.Unlock()
}

func createCSV(filename string) (*os.File, *csv.Writer) {
	f, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured during file creation. \n%v\n,", err)
		os.Exit(1)
	}
	writer := csv.NewWriter(f)
	writer.Write([]string{
		"ID", "Manufacturer", "Model", "Year", "Category",
		"Mileage", "FuelType", "EngineVolume", "DriveWheels", "GearBox", "Doors", "Wheel", "Color",
		"InteriorColor", "VIN", "LeatherInterior", "Price", "Clearance",
	})
	writer.Flush()
	return f, writer
}

func iterateOverPages(col *colly.Collector) {
	//25 per pages
	for i := 1; i < DealCount/25; i++ {
		col.Visit("https://www.myauto.ge/en/s/for-sale-cars?&currency_id=1&page=" + strconv.Itoa(i))
	}
}

func dealExtraction(dealsChannel chan *CarDeal, dc *colly.Collector) {
	// Extract details of a car deal
	dc.OnHTML(`div.container-main`, func(e *colly.HTMLElement) {
		aDeal := NewEmptyCarDeal()

		e.ForEach("th.th-left", func(_ int, el *colly.HTMLElement) {
			key := el.ChildText("div.th-key")
			value := el.ChildText("div.th-value")

			//Just in this case, symbols of whole sentence is different there
			engineKey := "undefined"
			if strings.Contains(key, "Engine") {
				engineKey = key
			}

			switch key {

			case "Manufacturer":
				aDeal.Manufacturer = value
			case "Model":
				aDeal.Model = value
			case "Prod. year":
				aDeal.Year = value
			case "Category":
				aDeal.Category = value
			case "Fuel type":
				aDeal.FuelType = value
			case engineKey:
				//Formating to have clean values (Turbo is irrelevant)
				if strings.Contains(value, "Turbo") {
					value = strings.ReplaceAll(value, "Turbo", "")
					value = strings.ReplaceAll(value, " ", "")
					value = strings.ReplaceAll(value, "\t", "")
					value = strings.ReplaceAll(value, "\n", "")
				}
				aDeal.EngineVolume = value
			case "Drive wheels":
				aDeal.DriveWheels = value
			case "Mileage":
				if strings.Contains(value, "km") {
					value = strings.ReplaceAll(value, " ", "")
					value = strings.ReplaceAll(value, "km", "")
				}
				aDeal.Mileage = value //Convert to uint
			case "Gear box type":
				aDeal.GearBox = value
			case "Doors":
				aDeal.Doors = value
			case "Wheel":
				aDeal.Wheel = value
			case "Color":
				aDeal.Color = value
			case "Interior color":
				aDeal.InteriorColor = value
			case "VIN":
				aDeal.VIN = value
			}

		})
		e.ForEach("th.th-right", func(_ int, el *colly.HTMLElement) {
			if el.ChildText("div.th-key") == "Leather interior" {
				aDeal.LeatherInterior = strings.Contains(el.ChildAttr("i", "class"), "fa-check")
			}
		})

		e.ForEach("label > span", func(_ int, el *colly.HTMLElement) {
			if strings.Contains(el.Text, "$") {
				extractedPrice := strings.ReplaceAll(strings.ReplaceAll(el.Text, " ", ""), "$", "")
				price, _ := strconv.Atoi(strings.ReplaceAll(extractedPrice, ",", ""))
				aDeal.Price = float32(price)
			}
		})

		e.ForEach("div.price", func(_ int, el *colly.HTMLElement) {
			aDeal.Clearance = strings.Contains(el.Text, "Customs-cleared")
		})

		e.ForEach("div.thumbnail-image > img", func(_ int, el *colly.HTMLElement) {
			aDeal.ImageURLs = append(aDeal.ImageURLs, el.Attr("src"))
		})

		//Sending already done deal to a channel
		dealsChannel <- aDeal
	})

}
