package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"scada-layout/dao"
	"scada-layout/model"
	layoutUtil "scada-layout/util"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/94peter/microservice/cfg"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/muulinCorp/interlib/util"
)

var env Env

func getTv(c *gin.Context) {
	confDir := env.ConfigDir

	inputId := c.Query("id")

	imgCut := map[string]*dao.ImageCut{}
	iconImgCutFilePath := util.StrAppend(confDir, "TvIconImageCut.yaml")
	err := layoutUtil.FileDecode(iconImgCutFilePath, imgCut, layoutUtil.YamlFileType)
	if err != nil {
		outputErr(c, err.Error() + " {icon image cut config decode failed}")
	}

	tvConfPattern := util.StrAppend(confDir, "Tv/*.yaml")
	tvConfFiles, err := filepath.Glob(tvConfPattern)
	if err != nil {
		outputErr(c, err.Error() + " {tv config files glob failed}")
	}
	if len(tvConfFiles) == 0 {
		outputErr(c, fmt.Sprintf("no tv config files found in %v", confDir))
	}

	var target *dao.Tv
	for _, file := range tvConfFiles {
		d := &model.TvConfig{}
		err := layoutUtil.FileDecode(file, d, layoutUtil.YamlFileType)
		if err != nil {
			outputErr(c, err.Error() + " {FileDecode failed}")
		}

		if d.Id == "" {
			continue
		}

		

		if inputId == "" {
			target = &dao.Tv{
				Id:    d.Id,
				ImageBaseUrl: env.StaticImagePath,
				Pages: getPages(d),
			}
			break
		}

		if inputId == d.Id {
			target = &dao.Tv{
				Id:    d.Id,
				ImageBaseUrl: env.StaticImagePath,
				Pages: getPages(d),
			}
			break
		}
	}

	if target == nil {
		c.String(http.StatusNotFound, "not found")
		return
	}

	if len(target.Pages) == 0 {
		outputErr(c, "tv config missing pages")
		return
	}

	c.JSON(http.StatusOK, target)
}

func getPages(d *model.TvConfig) []dao.Page {
	pages := []dao.Page{}
	for _, page := range d.Pages {
		err := page.Validation()
		if err != nil {
			log.Fatalf("Page: %s {%s}", page.Title, err)
		}

		tables := []dao.TableDetail{}
		for _, table := range page.Detail {
			err = table.Validation()
			if err != nil {
				log.Fatalf("Page: %s, Table: %s {%s}", page.Title, table.Title, err)
			}

			headers := []dao.HeaderDetail{}
			for _, header := range table.Header {
				headers = append(headers, dao.HeaderDetail{Title: header.Title})
			}

			cellDetails := [][]*dao.CellDetail{}
			for _, cells := range table.Detail {
				rows := []*dao.CellDetail{}

				for _, cell := range cells {
					err = cell.Validation()
					if err != nil {
						log.Fatalf("Page: %s, Table: %s, Cell: %s {%s}", page.Title, table.Title, cell.Title, err)
					}

					var value *string
					if cell.Value != nil {
						value = cell.Value
					}

					var displayDp int
					if cell.Dp != nil {
						displayDp = int(*cell.Dp)
					} else {
						displayDp = 0
					}

					rows = append(rows, &dao.CellDetail{
						Title:                  cell.Title,
						Field:                  cell.Field,
						DataAt:                 time.Now().Local().Format("2006-01-02 15:04:05"),
						Value:                  value,
						ValueUnit:              "",
						DisplayDp:              displayDp,
						Zero:                   nil,
						BitTrans:               nil,
						ValueTrans:             nil,
						MonitorValue:           cell.MonitorValue,
						MinValue:               cell.MinValue,
						LowerMinValueAlarmText: cell.MinValueAlarmText,
						MaxValue:               cell.MaxValue,
						OverMaxValueAlarmText:  cell.MaxValueAlarmText,
						Style: dao.CellStyle{
							TitleBackgroundColor: strings.ToUpper(page.Style.SubTitleBackgroundColor),
							TitleFontColor:       strings.ToUpper(page.Style.SubTitleFontColor),
							TitleFontSize:        cell.SubTitleFontSize,
							ValueBackgroundColor: strings.ToUpper(page.Style.ValueBackgroundColor),
							ValueFontColor:       strings.ToUpper(page.Style.ValueFontColor),
							ValueFontSize:        cell.ValueFontSize,
							DataAtFontColor:      strings.ToUpper(page.Style.DataAtFontColor),
							DataAtFontSize:       cell.DataAtFontSize,
							AlarmBackgroundColor: strings.ToUpper(page.Style.AlarmBackgroundColor),
							AlarmFontColor:       strings.ToUpper(page.Style.AlarmFontColor),
							AlarmFontSize:        cell.AlarmFontSize,
						},
						Icons: []*dao.CellIcon{},
					})
				}
				cellDetails = append(cellDetails, rows)
			}
			tables = append(tables, dao.TableDetail{
				Type: table.Type,
				Style: dao.TableStyle{
					Width:          table.Style.Width,
					Height:         table.Style.Height,
					TitleFontColor: strings.ToUpper(page.Style.TitleFontColor),
					TitleFontSize:  table.TitleFontSize,
					TitleBgColor:   strings.ToUpper(page.Style.TitleBackgroundColor),
				},
				Title:  table.Title,
				Header: headers,
				Detail: cellDetails,
				Footer: dao.FooterDetail{
					Style: dao.FooterStyle{
						ValueFontColor: strings.ToUpper(page.Style.FooterFontColor),
						ValueFontSize:  table.FooterFontSize,
					},
				},
			})
		}

		pages = append(pages, dao.Page{
			Title:       page.Title,
			DisplayTime: page.DisplayTime,
			Style: dao.PageTitleStyle{
				BackgroundColor: strings.ToUpper(page.Style.IndexTitleBackgroundColor),
				FontColor:       strings.ToUpper(page.Style.IndexTitleFontColor),
				FontSize:        page.Style.IndexTitleFontSize,
			},
			Detail: tables,
		})
	}

	return pages
}

func outputErr(c *gin.Context, err string) {
	c.String(http.StatusInternalServerError, err)
}

type Env struct {
	ConfigDir string `env:"CONF_DIR"` 
	StaticImagePath string `env:"STATIC_IMAGE_PATH"`
	ApiPort int `env:"API_PORT"`
}

func main() {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	envFile := path + "/.env"
	if util.FileExists(envFile) {
		err := godotenv.Load(envFile)
		if err != nil {
			log.Fatalln(err.Error() + " {load .env file fail}")
		}
	}
	err = cfg.GetFromEnv(&env)
	if err != nil {
		log.Fatalln(err.Error())
	}

	ginEngine := gin.New()
	ginEngine.GET("/web/layout/tv", getTv)
	//err = ginEngine.SetTrustedProxies([]string{"*"})
	port := ":" + strconv.Itoa(env.ApiPort)
	server := &http.Server{
		Addr:    port,
		Handler: ginEngine,
	}
	var apiWait sync.WaitGroup
	apiWait.Add(1)
	go func(srv *http.Server) {
		for {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s", err)
				time.Sleep(3 * time.Second)
			} else if err == http.ErrServerClosed {
				apiWait.Done()
				return
			}
		}
	}(server)

	apiWait.Wait()
	log.Println("api server exit")
}
