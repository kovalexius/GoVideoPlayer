//     Простой текстовый консольный видео-плеер.
//     Выводит кадры в символьном представлении на стандартный вывод.
//     Поддержки звука нет.
//
//     Как пользоваться:
//
//         $> go run simple-console-video-player.go /путь/до/видео-файла.mp4
//

package main

/*
#cgo pkg-config: libavcodec libavutil

// Импортируем данные из заголовочных файлов Cи для ffmpeg и libgmf.
// Далее будем обращаться к ним, через объект «C».

#include "libavcodec/avcodec.h"

*/
import "C"

import (
    "fmt"
    "os"
    "runtime/debug"
    "github.com/3d0c/gmf"
)

// Опишем константы
const (
    // Формат видео. Оттенки серого цвета от 0 до 2¹⁶ (65536).
    FORMAT              int32  = C.AV_PIX_FMT_GRAY16
    GRAY16_SIZE         int    = 65536
    // Кодек. Никакого кодека мы тут использовать не будем.
    CODEC_ID            int    = C.AV_CODEC_ID_RAWVIDEO
    // Представление пикселей с помощью символов псевдографики.
    TEXT_PIXEL_LIST     string = " .-+#"
    // Масштаб видео (делим на эти число).
    SCALE_FACTOR        int    = 4
    DEFAULT_FILE_NAME   string = "tests-sample.mp4"
)

func main() {
    srcFileName := DEFAULT_FILE_NAME
    if len(os.Args) > 1 {
        // Получаем имя видео-файла.
        // Если нам его не передали как аргумент командной строки,
        // то берем имя файла по-умолчанию.
        srcFileName = os.Args[1]
    }
    // Открываем его как видео-контейнер.
    inputContext, error := gmf.NewInputCtx(srcFileName)
    if error != nil {
        fatal(error)
    }
    // Говорим, что хотим закрыть контейнер,
    // когда закончим работу с ним
    defer inputContext.CloseInputAndRelease()
    // Выбираем видео-поток из контейнера.
    srcVideoStream, error := inputContext.GetBestStream(gmf.AVMEDIA_TYPE_VIDEO)
    if error != nil {
        fmt.Println("No video stream found in", srcFileName)
    }
    // Получим контекст (найтройки) кодека для потока исходного файла
    srcCodecContext := srcVideoStream.CodecCtx()
    // Получим размеры кадров для исходного файла.
    srcWidth  := srcCodecContext.Width()
    srcHeight := srcCodecContext.Height()
    // Вычислим размеры кадра, нужные нам.
    dstWidth  := srcWidth  / SCALE_FACTOR
    dstHeight := srcHeight / SCALE_FACTOR
    // Найдем нужный нам кодек (AV_CODEC_ID_RAWVIDEO).
    codec, error := gmf.FindEncoder(CODEC_ID)
    if error != nil {
        fatal(error)
    }
    // Создадим контекст кодека, и опишем его парпметры.
    dstCodecContext := gmf.NewCodecCtx(codec).
        SetPixFmt(FORMAT).
        SetWidth(dstWidth).
        SetHeight(dstHeight)
    // Говорим, что хотим освобидить память,
    // когда закончим работу с dstCodecContext.
    defer gmf.Release(dstCodecContext)
    // Иницализируем (откроем) контекст кодека.
    if error := dstCodecContext.Open(nil); error != nil {
        fatal(error)
    }
    // Создадим контекст масштаба.
    // Зададим исходный и результрующий контексты кодеков,
    // и то, как будем приводить один к другому (SWS_BICUBIC).
    scaleContext := gmf.NewSwsCtx(srcCodecContext,
                                  dstCodecContext,
                                  gmf.SWS_BICUBIC)
    // Говорим, что хотим освобидить память,
    // когда закончим работу с dstCodecContext.
    defer gmf.Release(scaleContext)
    // Создадим новый кадр и опишем его параметры:
    //  цветовую модель (FORMAT) и размеры.
    dstFrame := gmf.NewFrame().
        SetWidth(dstWidth).
        SetHeight(dstHeight).
        SetFormat(FORMAT)
    // Говорим, что хотим освобидить память,
    // когда закончим работу с dstFrame.
    defer gmf.Release(dstFrame)
    // Иницализируем кадр — выделяем для него память.
    if error := dstFrame.ImgAlloc(); error != nil {
        fatal(error)
    }
    // Извлекаем пакеты из видео-контейнера и проходим по кажому из них.
    for packet := range inputContext.GetNewPackets() {
        if packet.StreamIndex() != srcVideoStream.Index() {
            // Пропускаем не-видео кадры
            // (могут быть еще аудио-кадры или субтитры).
            continue
        }
        // Получаем поток пакета.
        packetStream, error := inputContext.GetStream(packet.StreamIndex())
        if error != nil {
            fatal(error)
        }
        // Получаем список кадров из пакета,
        // выполняем для каждого кадра из списка.
        for frame := range packet.Frames(packetStream.CodecCtx()) {
            // Масштабируем исходный кадр и результат кладем в dstFrame.
            scaleContext.Scale(frame, dstFrame)
            // Для полученного кадра пытаемя выделить последовательность байт,
            // и вывести их на консоль.
            if p, ready, _ := dstFrame.EncodeNewPacket(dstCodecContext); ready {
                // Выводим кадр в консоль.
                writeFrame(p.Data(), dstFrame.Width())
                defer gmf.Release(p)
            }
        }
        // Освобождаем память из под packet.
        gmf.Release(packet)
    }
}

// Выводит в консоль один кадр видео.
// Важный момент, что на вход функция принимает набор байтов,
// а не пикселей. При FORMAT = AV_PIX_FMT_GRAY16,
// один пиксель равен двум байтам.
func writeFrame(byteLinputStream []byte, width int) {
    pixel := 0
    for index, item := range byteLinputStream {
        // Формируем пиксель из текущего байта и предыдущего.
        pixel = 256 * pixel + int(item)
        if 1 == index % 2 {
            // Рисуем пиксель в консоль без перевода строки.
            fmt.Print(getGray16TextPixel(pixel))
            index = index / 2
            if 0 == (index + 1) % width {
                // Строка пикселей кончилась,
                // делаем перевод строки.
                fmt.Println()
            }
            pixel = 0
        }
    }
    // Конец кадра рисуем ограничитель (два перевода строки)
    fmt.Print("\n\n")
}

// Возвращает символ-пиксель, который соответствует,
// настоящему пикселю-числу формата «gray16».
func getGray16TextPixel(pixel int) string {
    // Получаем размер диапазона оттенков,
    // который будет представлен одним нашим текстовым «пикселем».
    // Эту переменную можно вынести из функции для повышения эффективности.
    step := int(GRAY16_SIZE) / len(TEXT_PIXEL_LIST)
    // Вычисляем номер нашего текстового пикселя.
    text_pixel_index := pixel / (step + 1)
    // Возвращаем символ из списка текстовых «пикселей»
    text_pixel := string(TEXT_PIXEL_LIST[text_pixel_index])
    return text_pixel
}

// Выводит подробный отчет об ошибке
func fatal(err error) {
    debug.PrintStack()
    fmt.Println(err)
}
