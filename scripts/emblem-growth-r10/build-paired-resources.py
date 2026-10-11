"""Rebuild paired badge resources in a new directory; never modify the input client.

Requires Python 3.10+, PowerShell 7, and the explicitly selected PVF editor DLL.
The editor is an external build dependency, like MSVC; it is not redistributed.
"""
from pathlib import Path
import argparse,base64,hashlib,json,shutil,struct,subprocess,zlib

ROOT=Path(__file__).resolve().parents[2]
RESOURCES=ROOT/'client-patchs/emblem-growth-r10/resources'
def sha(path):
 h=hashlib.sha256()
 with path.open('rb') as f:
  for data in iter(lambda:f.read(4*1024*1024),b''):h.update(data)
 return h.hexdigest()
def save(path,value):path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
def segments(data):
 at=0;rows=[]
 while at<len(data):
  start=at;count,n=struct.unpack_from('<II',data,at);at+=8;name=data[at:at+n].decode('ascii');at+=n
  for _ in range(count):n=struct.unpack_from('<I',data,at)[0];at+=4+n+8
  rows.append((name,data[start:at]))
 if at!=len(data):raise ValueError('Index trailing bytes')
 return rows
def images(base,out):
 manifest=json.loads((RESOURCES/'image-packs.json').read_text('utf-8'))
 for pack in manifest['packs']:
  path=out/'client'/pack['target'];path.parent.mkdir(parents=True,exist_ok=True)
  chunks={}
  for row in pack['images']:
   p=RESOURCES/row['file']
   if sha(p)!=row['sha256'] or p.stat().st_size!=row['size']:raise ValueError('Image changed: '+row['file'])
   chunks[row['offset']]=p.read_bytes()
  for gap in pack['gaps']:chunks[gap['offset']]=base64.b64decode(gap['raw_base64'])
  with path.open('wb') as f:
   f.write(bytes.fromhex(pack['header_hex']))
   for offset,data in sorted(chunks.items()):
    if f.tell()!=offset:raise ValueError('Pack offsets changed')
    f.write(data)
  if sha(path)!=pack['sha256']:raise ValueError('Pack identity changed')
 source=base/'ImagePacks2/NpkIndex.etc';index=manifest['index']
 if sha(source)!=index['base_sha256']:raise ValueError('Base image index changed')
 raw=source.read_bytes();plain=zlib.decompress(bytes(((((b^170)-7*i)&255)^170) for i,b in enumerate(raw[8:])))
 replacements={v['name']:base64.b64decode(v['raw_base64']) for v in index['replacement_records']}
 rows=[];used=set()
 for name,data in segments(plain):rows.append(replacements.get(name,data));used.add(name)
 for name,data in replacements.items():
  if name not in used:rows.append(data)
 plain=b''.join(rows);packed=zlib.compress(plain,9);encoded=bytes(((((b^170)+7*i)&255)^170) for i,b in enumerate(packed))
 target=out/'client/ImagePacks2/NpkIndex.etc';target.write_bytes(struct.pack('<II',len(plain),len(encoded))+encoded)
 if sha(target)!=index['result_sha256']:raise ValueError('Generated image index identity changed')
 return manifest
def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--base-client',type=Path,required=True,help='Read-only cumulative R7 resource directory')
 p.add_argument('--exe',type=Path,required=True)
 p.add_argument('--editor-dll',type=Path,required=True)
 p.add_argument('--pwsh',default='pwsh')
 p.add_argument('--out',type=Path,required=True,help='New empty output directory')
 args=p.parse_args();base=args.base_client.resolve();out=args.out.resolve()
 if out.exists() and any(out.iterdir()):raise ValueError('Output must be empty')
 if out==base or base in out.parents or out in base.parents:raise ValueError('Output overlaps input client')
 out.mkdir(parents=True,exist_ok=True)
 original={name:sha(base/name) for name in ('Script.pvf','sk.dat','ImagePacks2/NpkIndex.etc')}
 stages=[];source=base
 for stage in ('r2','r4','r5','r10'):
  meta=json.loads((RESOURCES/'stages'/f'{stage}.json').read_text('utf-8'));records=[]
  for ref in meta.pop('record_files'):
   path=RESOURCES/ref['file']
   if sha(path)!=ref['sha256']:raise ValueError('Source record changed: '+ref['file'])
   row=json.loads(path.read_text('utf-8'))
   if row['path']!=ref['path']:raise ValueError('Record identity changed')
   records.append(row)
  if stage=='r2' and original['Script.pvf']!=meta['client_script_sha256']:raise ValueError('R7 resource baseline mismatch')
  if stage=='r2' and original['sk.dat']!=meta['client_sk_sha256']:raise ValueError('R7 key baseline mismatch')
  meta['client_script_sha256']=sha(source/'Script.pvf');meta['client_sk_sha256']=sha(source/'sk.dat');meta['files']=records
  dest=out/'stages'/stage;dest.mkdir(parents=True);save(dest/'pvf-plan.json',meta)
  common=['-NoProfile','-File',str(Path(__file__).with_name('build-resources.ps1')),'-ClientDir',str(source),'-CandidateDir',str(dest),'-ExecutablePath',str(args.exe.resolve()),'-EditorDll',str(args.editor_dll.resolve())]
  subprocess.run([args.pwsh,*common],check=True)
  paths=json.loads((dest/'localized-paths.json').read_text('utf-8-sig'))
  for row in meta['localized']:row['destination_path']=paths[str(row['table'])]
  save(dest/'pvf-plan.json',meta)
  common[common.index(str(Path(__file__).with_name('build-resources.ps1')))]=str(Path(__file__).with_name('verify-resources.ps1'))
  subprocess.run([args.pwsh,*common],check=True)
  stages.append(json.loads((dest/'verification.json').read_text('utf-8-sig')));source=dest
 final=out/'client';final.mkdir(exist_ok=True)
 for name in ('Script.pvf','sk.dat'):shutil.copyfile(source/name,final/name)
 shutil.copyfile(source/'candidate.inner.pvf',out/'Script.inner.pvf')
 image_manifest=images(base,out)
 if original!={name:sha(base/name) for name in original}:raise ValueError('Input client changed during build')
 identities={name:sha(final/name) for name in ('Script.pvf','sk.dat','ImagePacks2/NpkIndex.etc')}
 identities['Script.inner.pvf']=sha(out/'Script.inner.pvf')
 save(out/'paired-resources.json',{'schema':1,'release':'emblem-growth-r10-git-candidate','files':identities,'stages':stages,'image_packs':[{'target':v['target'],'sha256':v['sha256']} for v in image_manifest['packs']],'input_client_unchanged':True,'manual_ui_accepted':False})
 print('PASS paired resource rebuild; input unchanged; visual acceptance pending',flush=True)
if __name__=='__main__':main()
