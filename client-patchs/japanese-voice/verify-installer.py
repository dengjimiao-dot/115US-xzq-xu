from pathlib import Path
import hashlib,json,subprocess,shutil,sys,uuid
sys.stdout.reconfigure(encoding='utf-8')
import argparse
parser=argparse.ArgumentParser(description='Validate voice installer only in an isolated fixture.')
parser.add_argument('--client',type=Path,required=True,help='Read-only original client for fixture source files.')
parser.add_argument('--out',type=Path,required=True,help='New disposable QA output directory.')
args=parser.parse_args()
REPO=Path(__file__).resolve().parents[2];PATCH=REPO/'client-patchs/japanese-voice'
ROOT=args.out.resolve()
if ROOT.exists():raise SystemExit('QA output already exists; choose a new directory.')
if ROOT==args.client.resolve() or args.client.resolve() in ROOT.parents:raise SystemExit('QA output must be outside the client.')
ROOT.mkdir();PKG=ROOT/'repo';PKG.mkdir();PACKAGE=PKG/'client-patchs/japanese-voice';PACKAGE.mkdir(parents=True);CLIENT=ROOT/'client';(CLIENT/'SoundPacks').mkdir(parents=True);(CLIENT/'DFO.exe').write_bytes(b'fixture-only')
manifest=json.loads((PATCH/'manifest.json').read_text('utf-8'))
rows=sorted(manifest['files'],key=lambda r:r['original_size'])[:2]
fixture=dict(manifest);fixture['files']=rows;fixture['baseline_pack_names']=[r['name'] for r in rows];fixture['default_client']=''
(PACKAGE/'manifest.json').write_text(json.dumps(fixture,ensure_ascii=False),encoding='utf-8')
(PKG/'scripts').mkdir();shutil.copy2(REPO/'scripts/jp-voice.ps1',PKG/'scripts/jp-voice.ps1');(PACKAGE/'payload/SoundPacks').mkdir(parents=True)
for r in rows:
 shutil.copy2(args.client/'SoundPacks'/r['name'],CLIENT/'SoundPacks'/r['name'])
 shutil.copy2(PATCH/'payload/SoundPacks'/r['name'],PACKAGE/'payload/SoundPacks'/r['name'])
def sha(p):
 with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def assert_state(patched):
 for r in rows:assert sha(CLIENT/'SoundPacks'/r['name'])==r['patched_sha256' if patched else 'original_sha256']
def quote(s):return "'"+str(s).replace("'","''")+"'"
results=[]
def run(mode,mock=False,expected=0,fault=False,force_running=False):
 cmd=[]
 if force_running:cmd.append('function Get-Process { [pscustomobject]@{ Id=12345 } }')
 if fault:
  cmd.append("$global:guardCalls=0; function Get-Process { $global:guardCalls++; if ($global:guardCalls -eq 5) { $global:held=[IO.File]::Open("+quote(CLIENT/'SoundPacks'/rows[1]['name'])+",[IO.FileMode]::Open,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None) }; @() }")
 elif mock:cmd.append('function Get-Process { @() }')
 cmd.append('& '+quote(PKG/'scripts/jp-voice.ps1')+' -Mode '+mode+' -ClientPath '+quote(CLIENT))
 p=subprocess.run(['powershell.exe','-NoProfile','-ExecutionPolicy','Bypass','-Command','; '.join(cmd)],capture_output=True)
 if p.returncode!=expected:raise AssertionError((mode,p.returncode,p.stdout.decode('utf-8',errors='replace'),p.stderr.decode('utf-8',errors='replace')))
 return p.stdout.decode('utf-8',errors='replace')
def passed(label):results.append(label);print('PASS',label,flush=True)
run('Check');assert_state(False);assert not (PACKAGE/'backups').exists();passed('只读校验不修改客户端')
run('Install',expected=1,force_running=True);assert_state(False);assert not (PACKAGE/'backups').exists();passed('游戏运行时拒绝安装')
alias=ROOT/'hardlink-original.npk'
p=subprocess.run(['powershell.exe','-NoProfile','-Command','New-Item -ItemType HardLink -Path '+quote(alias)+' -Target '+quote(CLIENT/'SoundPacks'/rows[0]['name'])+' | Out-Null'],capture_output=True);assert p.returncode==0
run('Install',mock=True);assert_state(True);assert sha(alias)==rows[0]['original_sha256'];passed('安装备份与原子替换，其他硬链接保留原数据')
pointer=json.loads((PACKAGE/'last-install.json').read_text('utf-8'));backup=Path(pointer['backup_path'])
for r in rows:assert sha(backup/'originals'/r['name'])==r['original_sha256']
passed('完整原文件备份校验')
before=len(list((PACKAGE/'backups').iterdir()));run('Install',mock=True);assert_state(True);assert len(list((PACKAGE/'backups').iterdir()))==before;passed('重复安装不重复写入')
run('Restore',mock=True);assert_state(False);run('Restore',mock=True);assert_state(False);passed('回退和重复回退逐字节恢复')
payload=PACKAGE/'payload/SoundPacks'/rows[0]['name'];good=payload.read_bytes();payload.write_bytes(good+b'corrupt');run('Install',mock=True,expected=1);assert_state(False);payload.write_bytes(good);passed('损坏补丁拒绝写入')
target=CLIENT/'SoundPacks'/rows[0]['name'];original=target.read_bytes();target.write_bytes(original+b'other-change');run('Install',mock=True,expected=1);assert target.read_bytes()==original+b'other-change';target.write_bytes(original);passed('原资源不匹配时保留其他修改')
target.write_bytes(good);run('Install',mock=True,expected=1);assert sha(target)==rows[0]['patched_sha256'];target.write_bytes(original);passed('部分安装状态拒绝继续覆盖')
run('Install',mock=True,expected=1,fault=True);assert_state(False);passed('中途锁定文件失败后自动恢复已写入文件')
pointer=json.loads((PACKAGE/'last-install.json').read_text('utf-8'));backup=Path(pointer['backup_path']);j=json.loads((backup/'installation.json').read_text('utf-8'));assert j['status']=='installation_failed'
target.write_bytes(good);run('Restore',mock=True);assert_state(False);passed('中断安装记录可以恢复部分安装')
run('Install',mock=True);assert_state(True);target.write_bytes(target.read_bytes()+b'other-after-install');run('Restore',mock=True,expected=1);assert target.read_bytes()==good+b'other-after-install';target.write_bytes(good);run('Restore',mock=True);assert_state(False);passed('回退前发现第三方改动则停止并保留')
unknown=CLIENT/'SoundPacks/sounds_thirdparty.npk';unknown.write_bytes(b'fixture');run('Install',mock=True,expected=1);assert_state(False);unknown.unlink();passed('检测额外语音补丁避免重名覆盖')
assert not list((CLIENT/'SoundPacks').glob('*.tmp'));passed('安装和回退无遗留暂存音频')
report=dict(result='passed',checks=results,fixture_root=str(ROOT),scope='Only isolated fixture files were installed/restored. Process guard mocked only for fixture tests, including running-game denial.',payload_files=len(manifest['files']),changed_audio=manifest['changed_entries'])
(ROOT/'verification.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
print('ALL',len(results),'CHECKS PASSED')
