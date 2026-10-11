param([Parameter(Mandatory=$true)][string]$ClientDir,[Parameter(Mandatory=$true)][string]$CandidateDir,[string]$ExecutablePath,[Parameter(Mandatory=$true)][string]$EditorDll)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
$editorDll=$EditorDll
[Reflection.Assembly]::LoadFrom($editorDll)|Out-Null
Add-Type -ReferencedAssemblies @($editorDll,'System.Runtime','System.Collections','System.IO','System.Console','System.Memory','System.Text.Json','System.Security.Cryptography','System.Text.Encoding','System.Text.Encoding.Extensions','System.Text.RegularExpressions') -TypeDefinition @'
using System;using System.IO;using System.Text;using System.Text.Json;using System.Text.RegularExpressions;using System.Collections.Generic;using System.Security.Cryptography;
public static class DualJobR3Build {
 static string Hash(string p){using var f=File.OpenRead(p);return Convert.ToHexString(SHA256.HashData(f)).ToLowerInvariant();}
 static byte[] Compile(A.m e,JsonElement ts,bool type7){using var m=new MemoryStream();using var w=new BinaryWriter(m);foreach(var t in ts.EnumerateArray()){byte k=t[0].GetByte();w.Write(k);w.Write(k==3||k==5||k==6||k==8||(k==7&&type7)?e.B(t[1].GetString()):t[1].GetInt32());}return m.ToArray();}
 public static void Run(string client,string output,string exe){
  using var doc=JsonDocument.Parse(File.ReadAllText(Path.Combine(output,"pvf-plan.json")));var plan=doc.RootElement;
  string script=Path.Combine(client,"Script.pvf"),sk=Path.Combine(client,"sk.dat");
  string scriptHash=plan.GetProperty("client_script_sha256").GetString(),skHash=plan.GetProperty("client_sk_sha256").GetString();
  if(Hash(script)!=scriptHash||Hash(sk)!=skHash||Hash(exe)!=plan.GetProperty("client_exe_sha256").GetString())throw new Exception("Client source identity changed");
  if(File.Exists(Path.Combine(output,"Script.pvf"))||File.Exists(Path.Combine(output,"sk.dat")))throw new Exception("Candidate already built");
  var e=new A.m(A.M.A(script,exe,sk));
  var changes=new List<string>();bool type7=plan.TryGetProperty("relocate_type7",out var t7)&&t7.GetBoolean();
  foreach(var f in plan.GetProperty("files").EnumerateArray()){
   string p=f.GetProperty("path").GetString();int kind=f.GetProperty("kind").GetInt32();
   byte[] raw;
   if(f.TryGetProperty("append_pairs",out var pairs)){
    int index=e.c(p);if(index<0)throw new Exception("Base index missing");byte[] old=e.Image.A(e.Image.A(index));
    if(old.Length!=f.GetProperty("base_pairs").GetInt32()*10)throw new Exception("Base index pair count changed");
    var ids=new HashSet<int>();for(int at=0;at<old.Length;at+=10){if(old[at]!=0||old[at+5]!=6)throw new Exception("Base index grammar changed");ids.Add(BitConverter.ToInt32(old,at+1));}
    using var stream=new MemoryStream();stream.Write(old);using var writer=new BinaryWriter(stream);
    foreach(var pair in pairs.EnumerateArray()){int id=pair[0].GetInt32();if(!ids.Add(id))throw new Exception("Duplicate stackable identity");writer.Write((byte)0);writer.Write(id);writer.Write((byte)6);writer.Write(e.B(pair[1].GetString()));}raw=stream.ToArray();
   }else raw=kind==1?Compile(e,f.GetProperty("tokens"),type7):Convert.FromBase64String(f.GetProperty("raw").GetString());
   e.A(p,kind,raw);changes.Add(p);
  }
  if(plan.TryGetProperty("append_index_pairs",out var append))foreach(var row in append.EnumerateArray()){
   string p=row.GetProperty("path").GetString();int index=e.c(p);if(index<0)throw new Exception("Append index missing: "+p);
   byte[] old=e.Image.A(e.Image.A(index));var ids=new HashSet<int>();for(int at=0;at<old.Length;at+=10){if(at+10>old.Length||old[at]!=0||old[at+5]!=6)throw new Exception("Index grammar: "+p);ids.Add(BitConverter.ToInt32(old,at+1));}
   using var stream=new MemoryStream();stream.Write(old);using var writer=new BinaryWriter(stream);
   foreach(var pair in row.GetProperty("pairs").EnumerateArray()){int id=pair[0].GetInt32();if(!ids.Add(id))throw new Exception("Duplicate equipment ID");writer.Write((byte)0);writer.Write(id);writer.Write((byte)6);writer.Write(e.B(pair[1].GetString()));}
   e.A(p,1,stream.ToArray());changes.Add(p);
  }
  int n=e.c("list/n_string.lst");if(n<0)throw new Exception("Native string index missing");
  byte[] idx=e.Image.A(e.Image.A(n));var paths=new Dictionary<int,string>();
  for(int at=0;at<idx.Length;at+=10){if(at+10>idx.Length||idx[at]!=0||idx[at+5]!=6)throw new Exception("Native string index grammar");paths.Add(BitConverter.ToInt32(idx,at+1),e.A(BitConverter.ToInt32(idx,at+6)));}
  File.WriteAllText(Path.Combine(output,"localized-paths.json"),JsonSerializer.Serialize(paths));
  var tables=new Dictionary<int,string>();
  foreach(var row in plan.GetProperty("localized").EnumerateArray()){
   int id=row.GetProperty("table").GetInt32();string key=row.GetProperty("key").GetString(),value=row.GetProperty("value").GetString();
   if(key.Length==0||key.Length>40||key.Contains(">")||key.Contains("\r")||key.Contains("\n"))throw new Exception("Native string tag grammar: "+key);
   if(!tables.TryGetValue(id,out string text)){if(!paths.TryGetValue(id,out string p))throw new Exception("String table ID absent "+id);int ix=e.c(p);if(ix<0)throw new Exception("String table absent "+p);text=Encoding.Unicode.GetString(e.Image.A(e.Image.A(ix)));}
   var rx=new Regex("(?m)^(?:\\uFEFF)?"+Regex.Escape(key)+">[^\\r\\n\\0]*");var matches=rx.Matches(text);
   if(matches.Count>1)throw new Exception("Duplicate target string key "+key);
   bool preserve=row.TryGetProperty("preserve_existing",out var keep)&&keep.GetBoolean();
   if(matches.Count==1){if(!preserve)text=rx.Replace(text,m=>(m.Value.StartsWith("\uFEFF",StringComparison.Ordinal)?"\uFEFF":"")+key+">"+value);}
   else {string tail=text.Substring(text.TrimEnd('\0').Length);text=text.TrimEnd('\0');if(!text.EndsWith("\n"))text+="\r\n";text+=key+">"+value+"\r\n"+tail;}
   tables[id]=text;
  }
  foreach(var row in tables){if(row.Value.IndexOf("\uFEFF",1,StringComparison.Ordinal)>=0)throw new Exception("Embedded BOM in string table "+paths[row.Key]);e.A(paths[row.Key],3,Encoding.Unicode.GetBytes(row.Value));changes.Add(paths[row.Key]);}
  e.A(Path.Combine(output,"Script.pvf"),Path.Combine(output,"sk.dat"),s=>Console.WriteLine(s));
  if(Hash(script)!=scriptHash||Hash(sk)!=skHash)throw new Exception("Original client unexpectedly changed");
  File.WriteAllText(Path.Combine(output,"build-report.json"),JsonSerializer.Serialize(new {changed=changes,script_sha256=Hash(Path.Combine(output,"Script.pvf")),sk_sha256=Hash(Path.Combine(output,"sk.dat")),original_client_unchanged=true},new JsonSerializerOptions{WriteIndented=true}));
 }
}
'@
if(!$ExecutablePath){$ExecutablePath=Join-Path $ClientDir 'DFO.exe'}
[DualJobR3Build]::Run([IO.Path]::GetFullPath($ClientDir),[IO.Path]::GetFullPath($CandidateDir),[IO.Path]::GetFullPath($ExecutablePath))
