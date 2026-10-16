"""Verify schema-2 install/uninstall in a new isolated fixture only."""
from pathlib import Path
import argparse,hashlib,json,os,shutil,subprocess,zipfile

PACKAGE=Path(__file__).resolve().parent
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--client',type=Path,required=True,help='Original client, read-only source of two fixture NPKs.')
parser.add_argument('--modkit',type=Path,required=True)
parser.add_argument('--out',type=Path,required=True)
args=parser.parse_args()
root=args.out.resolve();original_client=args.client.resolve()
if root.exists() or root==original_client or original_client in root.parents:
    raise SystemExit('Use a new fixture directory outside the client.')
root.mkdir(parents=True);client=root/'client';(client/'SoundPacks').mkdir(parents=True)
m=json.loads((PACKAGE/'manifest.json').read_text('utf-8'))
rows=sorted(m['files'],key=lambda row:row['original_size'])[:2]
ops=[]
for row in rows:
    name=row['name'];target='SoundPacks/'+name
    shutil.copy2(original_client/target,client/target)
    ops.append(dict(kind='file.replace',target=target,source='client/'+target,size=row['patched_size'],sha256=row['patched_sha256']))
organization=dict(schema=2,id='qol.japanese-voice',version='0.1.0-candidate',name='Japanese voice fixture',author='Deemo',permissions=['client.file.write'],layers={'client':{'ops':ops}})
archive=root/'fixture.mod.zip'
with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED) as z:
    z.writestr('mod.json',json.dumps(organization))
    for row in rows:z.write(PACKAGE/'payload/SoundPacks'/row['name'],'client/SoundPacks/'+row['name'])
alias=root/'original-hardlink.npk';os.link(client/'SoundPacks'/rows[0]['name'],alias)
def sha(p):
    with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
checks=[]
for mode,flags in [('verify',['--mod',str(archive)]),('plan',['--root',str(root),'--client',str(client),'--mod',str(archive)]),('install',['--root',str(root),'--client',str(client),'--mod',str(archive)]),('uninstall',['--root',str(root),'--client',str(client),'--id','qol.japanese-voice'])]:
    with (root/(mode+'.log')).open('wb') as log:
        p=subprocess.run([str(args.modkit.resolve()),mode,*flags],stdout=log,stderr=subprocess.STDOUT)
    if p.returncode:raise SystemExit(f'{mode} failed; see fixture log')
    expected='patched_sha256' if mode=='install' else 'original_sha256'
    for row in rows:assert sha(client/'SoundPacks'/row['name'])==row[expected]
    assert sha(alias)==rows[0]['original_sha256'],f'{mode}: other hardlink data changed'
    checks.append(mode);print('PASS',mode,flush=True)
(root/'verification.json').write_text(json.dumps(dict(result='passed',checks=checks,exact_restoration=True,other_hardlink_preserved=True),indent=2)+'\n',encoding='utf-8')
