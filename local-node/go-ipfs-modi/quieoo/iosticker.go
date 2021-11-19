package quieoo

import (
	"fmt"
	"github.com/ipfs/go-cid"
	"time"
)

type RequestLife struct{
	start time.Time
	finishResolve time.Time
	finishDownload time.Time
}
func (r *RequestLife) Start(t time.Time){
	r.start=t
}
func (r *RequestLife)FinishResolve(t time.Time){
	r.finishResolve=t
}
func (r *RequestLife)FinishDownload(t time.Time){
	r.finishDownload=t
}


type IOSticker struct {

	initialized bool

	LifeTime map[cid.Cid]RequestLife
}
func (s *IOSticker) CidStart(c cid.Cid,t time.Time){
	life := s.LifeTime[c]
	life.start=t
	s.LifeTime[c]=life
}
func(s *IOSticker) CidFinishResolve(c cid.Cid, t time.Time){
	life:=s.LifeTime[c]
	life.finishResolve=t
	s.LifeTime[c]=life
}
func(s *IOSticker) CidFinishDownload(c cid.Cid, t time.Time){
	life:=s.LifeTime[c]
	life.finishDownload=t
	s.LifeTime[c]=life
}


func(sticker *IOSticker) Run(){
	for{
		//fmt.Printf("Average IO spent time: %f s/MB\n",float64(sticker.ioTime.Seconds())/(float64(sticker.dataSize)/float64(1024*1024)))
		//fmt.Printf("Current Average Put Efficiency: %f MB/s\n",(float64(sticker.Putdatasize)/float64(1024*1024))/sticker.Puttime.Seconds())
		//fmt.Printf("current use time in put: %f s\n",sticker.Puttime.Seconds())
		fmt.Println("=============================================================================")
		for k,v:=range sticker.LifeTime{
			resolve:=v.finishResolve.Sub(v.start).Seconds()
			download:=v.finishDownload.Sub(v.finishResolve).Seconds()
			fmt.Printf("%s:(%f,%f)ms,%f\n",k,resolve*1000,download*1000,download/resolve)
		}
		time.Sleep(10*time.Second)
	}
}

func (stick *IOSticker) StartLogging(){
	//go stick.Run()
}



func NewIOSticker() *IOSticker{

	stick:=&IOSticker{
		LifeTime: make(map[cid.Cid]RequestLife),
	}

	//stick.StartLogging()
	return stick
}