package packer

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/Z3belek/Manga.azw3/cmd/crop"
	"github.com/cheggaaa/pb/v3"
)

type DiscoveryInput struct {
	rootDit string
}

type CommCh struct {
	done chan struct{}
	err  chan error
}

type FileData struct {
	filepath string
}

func fileFinder(commCh CommCh, rootDir string) <-chan FileData {
	outCh := make(chan FileData, 100)

	go func() {
		defer close(outCh)

		err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf(`error: problem ocurred in filepath.Walk callback: %w`, err)
			}

			if info.IsDir() {
				return nil
			}

			select {
			case outCh <- FileData{
				filepath: path,
			}:
			case <-commCh.done:
				return nil
			}

			return nil
		})

		if err != nil {
			commCh.err <- fmt.Errorf(`error walking the directory: %w`, err)
		}
	}()

	return outCh
}

func fileProcess(commCh CommCh, options ImageProcessor, inCh <-chan FileData) <-chan PageProcessor {
	outCh := make(chan PageProcessor, 100)

	var processPb = pb.New(0)
	processPb.Set("prefix", "Processing files")
	processPb.SetMaxWidth(80)
	processPb.Start()

	wg := sync.WaitGroup{}

	for i := 0; i < options.CoresCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for info := range inCh {
				processPb.AddTotal(1)

				originalImg, err := loadImage(info.filepath)

				if err != nil {
					log.Println(fmt.Errorf(`File "%v" is not an image: %w`, info.filepath, err))
					processPb.AddTotal(-1)
					continue
				}

				croppedImg := originalImg

				if !options.DisableCrop {
					if cropped, err := crop.Crop(originalImg, crop.Limits(originalImg, 0.1)); err != nil {
						commCh.err <- fmt.Errorf(`Error cropping image "%v": %w`, info.filepath, err)

						return
					} else {
						croppedImg = cropped
					}
				}

				finalImages := []image.Image{}

				bounds := croppedImg.Bounds()

				isDoublePage := bounds.Dx() >= bounds.Dy()

				if isDoublePage && options.DoublePage != "only-double" {
					leftImg, rightImg, err := crop.SplitImage(croppedImg)

					if err != nil {
						commCh.err <- fmt.Errorf(`Error splitting image "%v": %w`, info.filepath, err)
						return
					}

					switch options.DoublePage {
					case "only-split":
						finalImages = append(finalImages, rightImg, leftImg)
					case "split-then-double":
						finalImages = append(finalImages, rightImg, leftImg, croppedImg)
					case "double-then-split":
						finalImages = append(finalImages, croppedImg, rightImg, leftImg)
					default:
						if err != nil {
							commCh.err <- fmt.Errorf(`Error unknown double page option "%v"`, options.DoublePage)
							return
						}
						return
					}
				} else {
					finalImages = append(finalImages, croppedImg)
				}

				select {
				case outCh <- PageProcessor{
					imagePath: info.filepath,
					images:    finalImages,
				}:
					{
						processPb.Add(1)
					}
				case <-commCh.done:
					return
				}
			}
		}()
	}

	go func() {
		defer close(outCh)
		defer processPb.Finish()
		wg.Wait()
	}()

	return outCh
}

func loadImage(imgPath string) (image.Image, error) {
	img, err := os.Open(imgPath)
	if err != nil {
		return nil, err
	}
	defer img.Close()
	imgDecoded, _, err := image.Decode(img)
	return imgDecoded, err
}
