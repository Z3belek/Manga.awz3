package packer

import (
	"fmt"
	"hash/fnv"
	"image"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"facette.io/natsort"
	"github.com/cheggaaa/pb/v3"
	"github.com/leotaku/mobi"
	"github.com/leotaku/mobi/records"
)

type KindlePacker struct {
	RootDir        string
	DisableCrop    bool
	LeftToRight    bool
	DoublePage     string
	Title          string
	Author         string
	OutputFilePath string
	CoresCount     int
}

type ImageProcessor struct {
	DisableCrop bool
	DoublePage  string
	CoresCount  int
}

type PageProcessor struct {
	imagePath string
	images    []image.Image
}

func MangaPacker(params KindlePacker) error {
	commCh := CommCh{
		done: make(chan struct{}),
		err:  make(chan error),
	}

	defer close(commCh.done)
	defer close(commCh.err)

	pageProcessed := fileProcess(
		commCh,
		ImageProcessor{
			DisableCrop: params.DisableCrop,
			DoublePage:  params.DoublePage,
			CoresCount:  params.CoresCount,
		},
		fileFinder(commCh, params.RootDir),
	)

	go func() {
		for err := range commCh.err {
			if err != nil {
				log.Println(fmt.Errorf(`error: %w`, err))
				commCh.done <- struct{}{}
			}
		}
	}()

	processed := []PageProcessor{}

	for page := range pageProcessed {
		processed = append(processed, page)
	}

	sort.SliceStable(processed, func(i, j int) bool {
		a := processed[i].imagePath
		b := processed[j].imagePath
		aDir, _ := filepath.Split(a)
		bDir, _ := filepath.Split(b)
		sameDir := aDir == bDir

		if sameDir {
			return natsort.Compare(a, b)
		} else {
			if strings.Contains(bDir, aDir) {
				return true
			} else if strings.Contains(aDir, bDir) {
				return false
			} else {
				return natsort.Compare(aDir, bDir)
			}
		}
	})

	if len(processed) == 0 {
		return fmt.Errorf(`error: no images found in the directory`)
	}

	allImages := []image.Image{}
	bookChapters := []mobi.Chapter{}
	chapterBuffer := []string{}
	pageMangaIndex := 1
	prevChapter := getChapterByPath(processed[0].imagePath)

	for _, page := range processed {
		currentChapter := getChapterByPath(page.imagePath)
		isSameChapter := prevChapter == currentChapter

		if !isSameChapter {
			bookChapters = append(bookChapters, mobi.Chapter{
				Title:  prevChapter,
				Chunks: mobi.Chunks(chapterBuffer...),
			})
			chapterBuffer = []string{}
			prevChapter = currentChapter
		}

		for _, img := range page.images {
			allImages = append(allImages, img)
			chapterBuffer = append(chapterBuffer, templateStr(htmlTemplate, records.To32(pageMangaIndex)))
			pageMangaIndex++
		}
	}

	bookChapters = append(bookChapters, mobi.Chapter{
		Title:  prevChapter,
		Chunks: mobi.Chunks(chapterBuffer...),
	})

	mangaDirName := path.Base(params.RootDir)
	mangaTitle := params.Title

	if mangaTitle == "" {
		mangaTitle = mangaDirName
	}

	book := mobi.Book{
		Title:       mangaTitle,
		Authors:     []string{params.Author},
		CSSFlows:    []string{cssTemplate},
		Chapters:    bookChapters,
		Images:      allImages,
		CoverImage:  allImages[0],
		FixedLayout: true,
		RightToLeft: !params.LeftToRight,
		CreatedDate: time.Unix(0, 0),
		UniqueID:    getUniqueId(mangaTitle),
	}

	outputFilePath := params.OutputFilePath

	if outputFilePath == "" {
		outputFilePath = path.Join(params.RootDir, "../", mangaDirName+".azw3")
	}

	writer, err := os.Create(outputFilePath)

	if err != nil {
		return fmt.Errorf(`error: creating file "%v": %w`, outputFilePath, err)
	}

	mangaExport := pb.ProgressBarTemplate(`{{string . "prefix"}}{{counters . }} {{bar . }} {{percent . }} {{etime . }}`).Start64(int64(len(allImages)))
	mangaExport.Start()
	err = book.Realize().Write(writer)

	if err != nil {
		return fmt.Errorf(`error: writing azw3 file "%v": %w`, outputFilePath, err)
	}

	return nil
}

func getChapterByPath(imagePath string) string {
	return filepath.Base(filepath.Dir(imagePath))
}

func getUniqueId(title string) uint32 {
	hash := fnv.New32()

	hash.Write([]byte(title))

	return hash.Sum32()
}
