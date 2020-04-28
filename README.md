# MyAutoScrapper #
Scraps myauto.ge for car deals data

## Compilation instructions ##
- Clone this repo: *git clone https://github.com/Gogotchuri/MyAutoScrapper*
- cd into the root directory of this repository
- run command: *go build*

## Usage instuctions ##
- After compilation, run compiled binary: *./MyAutoScrapper*
- data.csv will be created or recreated in the directory, which contains all the data
- images folder will be created, which contains images of car deals, each subfolder is given an ID.
  And represents a single car deal. IDs in the csv match id's of image subfolders
## Data ##
[Dataset location on kaggle](https://www.kaggle.com/gogotchuri/myautogecardetails) <br>
Already scrapped data is described [here](https://github.com/Gogotchuri/MyAutoData)
