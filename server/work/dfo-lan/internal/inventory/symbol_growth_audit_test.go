package inventory

import (
 "dfolan/internal/catalog/pvf"
 "encoding/json"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

// Writes source evidence only; never reads or modifies a character store.
func TestAuditSymbolGrowthSource(t *testing.T) {
 p, dest := os.Getenv("SYMBOL_GROWTH_PVF"), os.Getenv("SYMBOL_GROWTH_EVIDENCE")
 if p == "" || dest == "" { t.Skip("explicit read-only source audit") }
 a,e:=pvf.OpenReadOnly(pvf.Options{Path:p,MaxBytes:1024*1024*1024},""); if e!=nil {t.Fatal(e)}; defer a.Close()
 paths:=[]string{}
 e=a.IterateFiles(func(f pvf.File) error {
  name:=strings.ToLower(f.ArchivePath)
  if !(strings.Contains(name,"emblemremodel/") || strings.HasPrefix(name,"etc/emblem/")) {return nil}
  paths=append(paths,f.ArchivePath)
  if !strings.HasSuffix(name,".ctp") && !strings.HasSuffix(name,".etc") && !strings.HasSuffix(name,".lst") {return nil}
  raw,err:=a.ReadRaw(f.ArchivePath); if err!=nil{return err}
  target:=filepath.Join(dest,filepath.FromSlash(f.ArchivePath)); if err=os.MkdirAll(filepath.Dir(target),0755);err!=nil{return err}
  if err=os.WriteFile(target,raw,0600);err!=nil{return err}
  var data any
  if strings.HasSuffix(name,".ctp") {data,err=pvf.ReadCTP(f.ArchivePath,raw)} else {data,err=a.Tokens(f.ArchivePath)}
  if err!=nil {return err}
  encoded,err:=json.MarshalIndent(data,"","  ");if err!=nil{return err}
  return os.WriteFile(target+".json",encoded,0600)
 });if e!=nil{t.Fatal(e)}
 b,_:=json.MarshalIndent(map[string]any{"source":a.Snapshot(),"paths":paths},"","  ")
 if e=os.WriteFile(filepath.Join(dest,"manifest.json"),b,0600);e!=nil {t.Fatal(e)}
 t.Logf("source %s; %d emblem-system paths",a.Snapshot().Checksum,len(paths))
}
