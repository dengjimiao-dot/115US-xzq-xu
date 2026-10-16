from pathlib import Path
import zipfile,struct,hashlib,json,shutil,sys,datetime,contextlib
sys.stdout.reconfigure(encoding='utf-8')
import argparse,subprocess,getpass,os
parser=argparse.ArgumentParser(description='Rebuild Japanese voice NPKs without writing to the client.')
parser.add_argument('--client',type=Path,required=True)
parser.add_argument('--jp-zip',type=Path,required=True)
parser.add_argument('--perfect-rar',type=Path,required=True)
parser.add_argument('--out',type=Path,required=True)
parser.add_argument('--sevenzip',default=r'C:\Program Files\7-Zip\7z.exe')
args=parser.parse_args()
OUT=args.out.resolve();BASE=args.client.resolve()/'SoundPacks'
if OUT==args.client.resolve() or args.client.resolve() in OUT.parents:raise SystemExit('Output must be outside the client.')
if OUT.exists():raise SystemExit('Output already exists; choose a new output directory.')
archive=args.jp_zip.resolve();perfect_archive=args.perfect_rar.resolve()
KEY=(b'puchikon@neople dungeon and fighter '+b'DNF'*100)[:256]
def sha(p):
 with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def index(p):
 with p.open('rb') as f:
  h=f.read(20);assert h[:16]==b'NeoplePack_Bill\0'
  n=struct.unpack_from('<I',h,16)[0];assert 0<n<200000
  t=f.read(n*264);assert len(t)==n*264
  rows=[]
  for i in range(n):
   off,size=struct.unpack_from('<II',t,i*264)
   raw=t[i*264+8:(i+1)*264]
   name=bytes(x^y for x,y in zip(raw,KEY)).split(b'\0')[0].decode('latin1')
   assert off>=20+n*264 and off+size<=p.stat().st_size
   rows.append(dict(name=name,offset=off,size=size,raw=raw))
  return h,rows
OUT.mkdir(parents=True,exist_ok=True)
WORK=OUT/'.build';WORK.mkdir(exist_ok=True)
PAYLOAD=OUT/'payload/SoundPacks';PAYLOAD.mkdir(parents=True,exist_ok=True)
password=os.environ.get('JPVOICE_RAR_PASSWORD') or getpass.getpass('RAR password: ')
perfect_root=WORK/'perfect'
subprocess.run([args.sevenzip,'x','-y','-bso0','-bsp0','-p'+password,'-o'+str(perfect_root),str(perfect_archive)],check=True)
for i,p in enumerate(sorted(perfect_root.rglob('*.rar'))):
 subprocess.run([args.sevenzip,'x','-y','-bso0','-bsp0','-p'+password,'-o'+str(perfect_root/f'nested-{i}'),str(p)],check=True)
password=None
source=WORK/'japanese.npk'
with zipfile.ZipFile(archive) as z:
 entry=next(e for e in z.infolist() if e.filename.lower().endswith('.npk'))
 if not source.exists():
  with z.open(entry) as r,source.open('wb') as w:shutil.copyfileobj(r,w,4*1024*1024)
 for e in z.infolist():
  if e.filename.endswith('.txt'):(OUT/'日服原作者更新说明.txt').write_bytes(z.read(e))
_,src=index(source)
lookup={}
with source.open('rb') as f:
 for r in src:
  r['path']=source
  old=lookup.get(r['name'].lower())
  if old:
   f.seek(old['offset']);a=f.read(old['size']);f.seek(r['offset']);assert a==f.read(r['size']),r['name']
  else:lookup[r['name'].lower()]=r
for r in src:assert r['name'].lower().startswith('sounds/') and r['name'].lower().endswith('.ogg')
primary_names=set(lookup);skipped_invalid=[];perfect_added=set()
for p in sorted((WORK/'perfect').rglob('*.npk')):
 _,rr=index(p)
 with p.open('rb') as f:
  for r in rr:
   f.seek(r['offset']);magic=f.read(min(4,r['size']))
   if magic!=b'OggS':skipped_invalid.append(dict(pack=p.name,name=r['name'],size=r['size']));continue
   if r['name'].lower() not in lookup:
    assert r['name'].lower().startswith('sounds/') and r['name'].lower().endswith('.ogg')
    r['path']=p;lookup[r['name'].lower()]=r;perfect_added.add(r['name'].lower())
files=[];total_match=total_changed=0;used=set()
with contextlib.ExitStack() as stack:
 streams={p:stack.enter_context(p.open('rb')) for p in set(r['path'] for r in lookup.values())}
 for base in sorted(BASE.glob('*.npk')):
  h,rows=index(base);matches=sum(r['name'].lower() in lookup for r in rows)
  if not matches:continue
  dest=PAYLOAD/base.name;table=bytearray();offset=20+264*len(rows)+32;changes=0
  with base.open('rb') as bf,dest.open('w+b') as out:
   out.seek(offset)
   for r in rows:
    bf.seek(r['offset']);old=bf.read(r['size']);assert len(old)==r['size']
    selected=lookup.get(r['name'].lower())
    if selected:
     sf=streams[selected['path']]
     sf.seek(selected['offset']);data=sf.read(selected['size']);assert len(data)==selected['size']
     assert data[:4]==b'OggS',r['name']
     used.add(r['name'].lower())
    else:data=old
    changes+=data!=old
    table+=struct.pack('<II',offset,len(data))+r['raw'];out.write(data);offset+=len(data)
   header=h+table;signature=hashlib.sha256(header[:len(header)//17*17]).digest()
   out.seek(0);out.write(header+signature)
  # Independent reread checks all names, bounds, each selected audio and preserved audio.
  nh,new=index(dest);assert nh==h and len(new)==len(rows)
  with dest.open('rb') as nf,base.open('rb') as bf:
   check_header=nf.read(20+264*len(rows));assert nf.read(32)==hashlib.sha256(check_header[:len(check_header)//17*17]).digest()
   for a,b in zip(rows,new):
    assert a['name']==b['name'] and a['raw']==b['raw']
    nf.seek(b['offset']);actual=nf.read(b['size']);selected=lookup.get(a['name'].lower())
    if selected:
     sf=streams[selected['path']];sf.seek(selected['offset']);expected=sf.read(selected['size'])
    else:bf.seek(a['offset']);expected=bf.read(a['size'])
    assert actual==expected,a['name']
  if not changes:dest.unlink();continue
  files.append(dict(name=base.name,original_sha256=sha(base),patched_sha256=sha(dest),original_size=base.stat().st_size,patched_size=dest.stat().st_size,entries=len(rows),matched=matches,changed=changes))
  total_match+=matches;total_changed+=changes
  print(f'{base.name}: matched {matches}, changed {changes}',flush=True)
manifest=dict(schema=1,created_local='2026-10-11',default_client='',source_archive=archive.name,source_sha256=sha(archive),supplement_archive=perfect_archive.name,supplement_sha256=sha(perfect_archive),source_entries=len(src),unique_combined_entries=len(lookup),matched_entries=len(used),primary_matched=len(used&primary_names),perfect_supplement_matched=len(used&perfect_added),unmatched_entries=len(lookup)-len(used),changed_entries=total_changed,files=files,baseline_pack_names=[p.name for p in BASE.glob('*.npk')],verification='Every entry reread; replaced bytes match source; unreplaced bytes match original; native index signatures verified.',excluded=['韩服语音补丁补充包-米歇尔攻坚战版本.zip：韩语补充，默认不混入日服补丁'],skipped_invalid_audio=skipped_invalid,merge_rule='Latest Japanese zip wins; valid Ogg audio from perfect Japanese rar fills missing names; zero-filled mute stubs are excluded.')
(OUT/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(dict(output=str(OUT),files=len(files),matched=len(used),changed=total_changed,payload_bytes=sum(f['patched_size'] for f in files)),ensure_ascii=False),flush=True)
