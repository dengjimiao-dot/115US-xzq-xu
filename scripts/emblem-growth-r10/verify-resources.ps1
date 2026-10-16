param([Parameter(Mandatory=$true)][string]$CandidateDir,[Parameter(Mandatory=$true)][string]$ClientDir,[string]$ExecutablePath,[Parameter(Mandatory=$true)][string]$EditorDll)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
$dll=$EditorDll
[Reflection.Assembly]::LoadFrom($dll)|Out-Null
Add-Type -ReferencedAssemblies @($dll,'System.Runtime','System.Collections','System.Console','System.IO','System.Memory','System.Text.Json','System.Security.Cryptography','System.Text.Encoding','System.Text.Encoding.Extensions') -TypeDefinition @'
using System;using System.IO;using System.Text;using System.Text.Json;using System.Collections.Generic;using System.Security.Cryptography;
public static class DualJobR3Verify {
 static string Hash(byte[] b)=>Convert.ToHexString(SHA256.HashData(b)).ToLowerInvariant();
 static void Check(bool value,string message){if(!value)throw new Exception(message);}
 static string PathAt(A.M a,A.E pool,int i){int at=a.FileTableOffset+i*24;return (pool.A(BitConverter.ToInt32(a.Stream,at+4))+"/"+pool.A(BitConverter.ToInt32(a.Stream,at))).TrimStart('/').ToLowerInvariant();}
 public static void Run(string output,string client,string exe){
  using var doc=JsonDocument.Parse(File.ReadAllText(Path.Combine(output,"pvf-plan.json")));var plan=doc.RootElement;
  Check(Hash(File.ReadAllBytes(Path.Combine(client,"Script.pvf")))==plan.GetProperty("client_script_sha256").GetString(),"US identity changed");
  Check(Hash(File.ReadAllBytes(Path.Combine(client,"sk.dat")))==plan.GetProperty("client_sk_sha256").GetString(),"Source sk identity changed");
  Check(Hash(File.ReadAllBytes(exe))==plan.GetProperty("client_exe_sha256").GetString(),"Executable identity changed");
  var before=A.M.A(Path.Combine(client,"Script.pvf"),exe,Path.Combine(client,"sk.dat"));
  var after=A.M.A(Path.Combine(output,"Script.pvf"),exe,Path.Combine(output,"sk.dat"));
  Check(after.EntryCount>=before.EntryCount,"entries lost");
  Check(after.AnsiPool.AsSpan(0,before.AnsiPool.Length).SequenceEqual(before.AnsiPool),"old ANSI pool changed");
  Check(after.UnicodePool.AsSpan(0,before.UnicodePool.Length).SequenceEqual(before.UnicodePool),"old Unicode pool changed");
  Check(after.Stream.AsSpan(after.BodyOffset,before.BodySize).SequenceEqual(before.Stream.AsSpan(before.BodyOffset,before.BodySize)),"old compressed body changed");
  // Group index entries include original byte endpoints and uncompressed sizes.
  Check(after.Stream.AsSpan(after.GroupOffset,before.GroupCount*8).SequenceEqual(before.Stream.AsSpan(before.GroupOffset,before.GroupCount*8)),"old group metadata changed");
  var expected=new Dictionary<string,JsonElement>(StringComparer.OrdinalIgnoreCase);var changed=new HashSet<string>(StringComparer.OrdinalIgnoreCase);
  foreach(var row in plan.GetProperty("files").EnumerateArray()){string p=row.GetProperty("path").GetString();expected.Add(p,row);changed.Add(p);}
  foreach(var row in plan.GetProperty("localized").EnumerateArray())changed.Add(row.GetProperty("destination_path").GetString());
  if(plan.TryGetProperty("append_index_pairs",out var appended))foreach(var row in appended.EnumerateArray())changed.Add(row.GetProperty("path").GetString());
  var bp=before.C();var ap=after.C();var oldChanged=new List<string>();long unchanged=0;
  for(int i=0;i<before.EntryCount;i++){
   if(before.Stream.AsSpan(before.FileTableOffset+i*24,24).SequenceEqual(after.Stream.AsSpan(after.FileTableOffset+i*24,24))){unchanged++;continue;}
   string path=PathAt(before,bp,i);Check(changed.Contains(path),"unexpected existing record changed: "+path);
   Check(path==PathAt(after,ap,i),"old path changed: "+path);oldChanged.Add(path);
  }
  var archive=USLocalServer.Pvf.PvfArchive.OpenClient110(Path.Combine(output,"Script.pvf"),exe,Path.Combine(output,"sk.dat"));
  if(plan.TryGetProperty("append_index_pairs",out var appendRows))foreach(var row in appendRows.EnumerateArray()){
   string path=row.GetProperty("path").GetString();int index=-1;
   for(int i=0;i<before.EntryCount;i++){if(PathAt(before,bp,i)==path){index=i;break;}}Check(index>=0,"old append index missing");
   byte[] old=before.A(before.A(index)),current=archive.ReadRaw(path);Check(current.AsSpan(0,old.Length).SequenceEqual(old),"old index pairs changed");int at=old.Length;
   foreach(var pair in row.GetProperty("pairs").EnumerateArray()){Check(current[at]==0&&BitConverter.ToInt32(current,at+1)==pair[0].GetInt32()&&current[at+5]==6&&archive.Strings.A(BitConverter.ToInt32(current,at+6))==pair[1].GetString(),"appended pair mismatch");at+=10;}Check(at==current.Length,"append trailing bytes");
  }
  int strings=0,tokens=0;bool type7=plan.TryGetProperty("relocate_type7",out var t7)&&t7.GetBoolean();
  foreach(var pair in expected){
   var row=pair.Value;byte[] raw=archive.ReadRaw(pair.Key);int at=0;
   if(row.TryGetProperty("append_pairs",out var pairs)){
    int index=-1;for(int i=0;i<before.EntryCount;i++)if(PathAt(before,bp,i)==pair.Key){index=i;break;}Check(index>=0,"Old index missing");
    byte[] old=before.A(before.A(index));Check(raw.AsSpan(0,old.Length).SequenceEqual(old),"Native index prefix changed");int tail=old.Length;
    foreach(var item in pairs.EnumerateArray()){Check(raw[tail]==0&&BitConverter.ToInt32(raw,tail+1)==item[0].GetInt32()&&raw[tail+5]==6&&archive.Strings.A(BitConverter.ToInt32(raw,tail+6))==item[1].GetString(),"Appended stackable mismatch");tail+=10;}Check(tail==raw.Length,"Index trailing bytes");continue;
   }
   if(row.GetProperty("kind").GetInt32()!=1){Check(raw.AsSpan().SequenceEqual(Convert.FromBase64String(row.GetProperty("raw").GetString())),"raw file changed: "+pair.Key);continue;}
   foreach(var t in row.GetProperty("tokens").EnumerateArray()){
    int kind=t[0].GetInt32();Check(at+5<=raw.Length&&raw[at]==kind,"token kind/length: "+pair.Key+"@"+at);
    int value=BitConverter.ToInt32(raw,at+1);
    if(kind==3||kind==5||kind==6||kind==8||(kind==7&&type7)){Check(archive.Strings.A(value)==t[1].GetString(),"pool remap: "+pair.Key+"@"+at);strings++;}
    else Check(value==t[1].GetInt32(),"raw number changed: "+pair.Key+"@"+at);
    at+=5;tokens++;
   }
   Check(at==raw.Length,"trailing tokens: "+pair.Key);
  }
  var texts=new Dictionary<string,string>(StringComparer.OrdinalIgnoreCase);
  foreach(var row in plan.GetProperty("localized").EnumerateArray()){
   string p=row.GetProperty("destination_path").GetString();if(!texts.TryGetValue(p,out string text)){text=Encoding.Unicode.GetString(archive.ReadRaw(p));texts.Add(p,text);}
   Check(text.Contains(row.GetProperty("key").GetString()+">"+row.GetProperty("value").GetString()+"\r\n"),"missing localized value");
  }
  // All original string rows are preserved byte-for-byte as an initial prefix,
  // excluding only their optional terminal NULs before appended private rows.
  foreach(var pair in texts){
   int index=-1;for(int i=0;i<before.EntryCount;i++){if(PathAt(before,bp,i)==pair.Key.ToLowerInvariant()){index=i;break;}}
   Check(index>=0,"old string table missing");byte[] old=before.A(before.A(index));string oldText=Encoding.Unicode.GetString(old).TrimEnd('\0');var permitted=new HashSet<string>();foreach(var l in plan.GetProperty("localized").EnumerateArray())if(l.GetProperty("destination_path").GetString()==pair.Key)permitted.Add(l.GetProperty("key").GetString());
   var newLines=new HashSet<string>(pair.Value.Split(new[]{"\r\n","\n"},StringSplitOptions.None));foreach(string line in oldText.Split(new[]{"\r\n","\n"},StringSplitOptions.None)){int split=line.IndexOf('>');if(split>=0&&!permitted.Contains(line.Substring(0,split)))Check(newLines.Contains(line),"Unrequested localization changed");}
  }
  string inner=Path.Combine(output,"candidate.inner.pvf");File.WriteAllBytes(inner,after.Stream);
  var result=new {success=true,old_record_count=before.EntryCount,new_record_count=after.EntryCount,unchanged_records=unchanged,changed_existing_records=oldChanged,original_pool_prefixes_identical=true,original_compressed_body_identical=true,original_group_metadata_identical=true,expected_files_checked=expected.Count,semantic_tokens_checked=tokens,semantic_strings_checked=strings,localization_tables_checked=texts.Count,inner_path=inner,inner_sha256=Hash(after.Stream),script_sha256=Hash(File.ReadAllBytes(Path.Combine(output,"Script.pvf"))),sk_sha256=Hash(File.ReadAllBytes(Path.Combine(output,"sk.dat"))),native_gameplay_verified=false};
  File.WriteAllText(Path.Combine(output,"verification.json"),JsonSerializer.Serialize(result,new JsonSerializerOptions{WriteIndented=true}));Console.WriteLine(JsonSerializer.Serialize(result));
 }
}
'@
if(!$ExecutablePath){$ExecutablePath=Join-Path $ClientDir 'DFO.exe'}
[DualJobR3Verify]::Run([IO.Path]::GetFullPath($CandidateDir),[IO.Path]::GetFullPath($ClientDir),[IO.Path]::GetFullPath($ExecutablePath))
