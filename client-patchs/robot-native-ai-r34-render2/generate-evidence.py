from pathlib import Path
import sys,json,hashlib,re
root=Path(__file__).resolve().parent

import pefile,capstone
exe=Path(sys.argv[1]) if len(sys.argv)>1 else sys.exit('Usage: python generate-evidence.py PATH_TO_DFO_EXE');sha=hashlib.sha256(exe.read_bytes()).hexdigest()
assert sha=='515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb'
p=pefile.PE(str(exe),fast_load=True);md=capstone.Cs(capstone.CS_ARCH_X86,capstone.CS_MODE_64)
md.detail=True
hooks=[('Frame',0x5c73760,5),('Destroy',0x642a970,5),('Ai',0x5c70760,6),('Local',0x5c70f30,6),('Attack',0x5e6fd10,13),('Range',0x2e60350,5),('AttackRange',0x2e60460,5),('Skill',0x5ef9270,6)]
hooks += [('SkillLookup',0x5ef4430,6),('BackdropCoordinates',0x5b8ac50,6)]
hooks += [('GoalX',0x5e7aba0,6),('GoalY',0x5e7abb0,6)]
hooks += [('Move',0x5e7b330,6),('Value',0x5e65100,7),('Awakening',0x2ed1f00,6),('Tick',0x5c72ab0,5)]
hooks += [('PartyBuff',0x2e60f00,5),('Charging',0x2e60880,5),('HasDelay',0x2e60b20,5),('Delay',0x2e5f550,5),('HasReuse',0x2e61050,5),('Reuse',0x2e601f0,5)]
hooks += [('HasJump',0x2e609d0,5),('Jump',0x2e5ead0,5)]
hooks += [('Character',0x5c6dc10,6)]
hooks += [('BaseStats',0x5c6b610,6),('ResetStats',0x5c11d20,6),('Recalculate',0x5cce150,6),('PetPreloadCall1',0x5e47a7b,5),('PetPreloadCall2',0x5e47e9b,5)]
hooks += [('PartyHud',0x5553590,6),('PartyHudCompact',0x5542930,6),('CoinDraw',0x5552be0,6)]
hooks += [('DestroyFighter', 104218016, 5), ('DestroyGunner', 104278720, 5), ('DestroyMage', 104614592, 5), ('DestroyPriest', 104897344, 5), ('DestroyFemaleGunner', 3403328, 5), ('DestroyThief', 105155728, 5)]
hooks += [('CommonDestroy',0x5787110,5)]
binding={'client_sha256':sha,'hooks':{},'functions':{}}
out=['#pragma once','#include <bcrypt.h>','#pragma comment(lib,"bcrypt.lib")']
for name,rva,n in hooks:
    raw=p.get_data(rva,n);ins=list(md.disasm(raw,0x140000000+rva))
    assert sum(i.size for i in ins)==n
    if name.startswith('PetPreloadCall'):
        assert len(ins)==1 and ins[0].mnemonic=='call' and ins[0].operands[0].imm==0x145e47f00
    else:
        assert all(i.mnemonic in ('mov','push','sub','cmp') and 'rip' not in i.op_str for i in ins)
    out.append(f'static const unsigned char k{name}Head[]={{'+','.join(hex(x) for x in raw)+'};')
    binding['hooks'][name]={'rva':hex(rva),'bytes':raw.hex(),'instructions':[i.mnemonic+' '+i.op_str for i in ins]}
out.append('struct NativeHookBinding {uintptr_t rva;const unsigned char* bytes;size_t size;bool call;};')
out.append('static const NativeHookBinding kAllNativeHooks[]={')
for name,rva,n in hooks:
    out.append('{'+hex(rva)+',k'+name+'Head,sizeof(k'+name+'Head),'+('true' if name.startswith('PetPreloadCall') else 'false')+'},')
out.append('};')
source='\n'.join(x.read_text() for x in sorted((root/'src').glob('*.cpp')))
rvas={int(x,16) for x in re.findall(r'Call\((?:b,)?\s*(0x[0-9a-f]+)\s*[,)]',source)}
rvas.update(x[1] for x in hooks)
classes=[(181539696, 107000, 105027312), (181423608, 105216, 104215536), (181441832, 118344, 104273824), (181470704, 107920, 104610784), (181505624, 106640, 104894816), (153264040, 118224, 3397040), (181557104, 105472, 105154512), (181398360, 108096, 103816992), (171741880, 103912, 75077648), (168022640, 107352, 61930384), (169219608, 104016, 65358336), (174579664, 104240, 82108288), (173661632, 103968, 79559680), (170387872, 104000, 70718208), (168040864, 104248, 62074800), (166182688, 104016, 56840864), (156948248, 104320, 16976720)]
rvas.update(c[2] for c in classes)
rvas.update((0x1b07610,0x642b440,0x5c658d0,0x5e67c2a))
out.append('static bool VerifyNativeImage(uintptr_t base) {')
out.extend([
' auto dos=(IMAGE_DOS_HEADER*)base;',
' if(!Readable(dos,sizeof(*dos))||dos->e_magic!=IMAGE_DOS_SIGNATURE||dos->e_lfanew<0||dos->e_lfanew>0x100000)return false;',
' auto nt=(IMAGE_NT_HEADERS64*)(base+dos->e_lfanew);',
f' if(!Readable(nt,sizeof(*nt))||nt->Signature!=IMAGE_NT_SIGNATURE||nt->FileHeader.TimeDateStamp!={p.FILE_HEADER.TimeDateStamp}u||nt->OptionalHeader.SizeOfImage!={p.OPTIONAL_HEADER.SizeOfImage}u)return false;'
])
for rva in sorted(rvas):
    raw=p.get_data(rva,16);assert len(raw)==16
    binding['functions'][hex(rva)]=raw.hex()
    out.append(' { const unsigned char bytes[]={'+','.join(hex(x) for x in raw)+'};'+f' if(!Readable((void*)(base+{hex(rva)}),sizeof(bytes))||memcmp((void*)(base+{hex(rva)}),bytes,sizeof(bytes)))return false; }}')
import struct
binding['classes']=[]
for job,(vt,size,ctor) in enumerate(classes):
    methods={}
    for off in (0,0x90,0x570,0x12e0,0x12f0,0x2338):
        target=struct.unpack('<Q',p.get_data(vt+off,8))[0]-p.OPTIONAL_HEADER.ImageBase
        assert 0<target<p.OPTIONAL_HEADER.SizeOfImage
        methods[hex(off)]=hex(target)
        out.append(f' if(!Readable((void*)(base+{hex(vt+off)}),8)||*(uintptr_t*)(base+{hex(vt+off)})!=base+{hex(target)})return false;')
    binding['classes'].append({'job':job,'vtable_rva':hex(vt),'size':hex(size),'constructor_rva':hex(ctor),'methods':methods})
for vt,offsets in ((0x9c4f688,(0x60,0x130)),(0x9a46f30,(0x188,0x190))):
 for off in offsets:
  target=struct.unpack('<Q',p.get_data(vt+off,8))[0]-p.OPTIONAL_HEADER.ImageBase
  out.append(f' if(!Readable((void*)(base+{hex(vt+off)}),8)||*(uintptr_t*)(base+{hex(vt+off)})!=base+{hex(target)})return false;')
  binding.setdefault('render_vtables',{}).setdefault(hex(vt),{})[hex(off)]=hex(target)
out.extend([
' wchar_t path[1024];DWORD n=GetModuleFileNameW(nullptr,path,_countof(path));if(!n||n>=_countof(path))return false;',
' HANDLE file=CreateFileW(path,GENERIC_READ,FILE_SHARE_READ,nullptr,OPEN_EXISTING,FILE_ATTRIBUTE_NORMAL,nullptr);if(file==INVALID_HANDLE_VALUE)return false;',
' BCRYPT_ALG_HANDLE alg=nullptr;BCRYPT_HASH_HANDLE hash=nullptr;unsigned char buffer[65536],digest[32];bool ok=false;DWORD got=0;',
' if(BCryptOpenAlgorithmProvider(&alg,BCRYPT_SHA256_ALGORITHM,nullptr,0)>=0&&BCryptCreateHash(alg,&hash,nullptr,0,nullptr,0,0)>=0) {',
'  ok=true;for(;;){if(!ReadFile(file,buffer,sizeof(buffer),&got,nullptr)){ok=false;break;}if(!got)break;if(BCryptHashData(hash,buffer,got,0)<0){ok=false;break;}}',
'  ok=ok&&BCryptFinishHash(hash,digest,sizeof(digest),0)>=0;',
' }',
' if(hash)BCryptDestroyHash(hash);if(alg)BCryptCloseAlgorithmProvider(alg,0);CloseHandle(file);',
' const unsigned char expected[]={'+','.join('0x'+sha[i:i+2] for i in range(0,64,2))+'};',
' return ok&&!memcmp(digest,expected,32);','}'
])
dist=root/'dist';dist.mkdir(exist_ok=True)
(dist/'native_evidence.h').write_text('\n'.join(out)+'\n')
binding['sources']={str(x.relative_to(root)).replace('\\','/'):hashlib.sha256(x.read_bytes()).hexdigest() for x in sorted((root/'src').glob('*')) if x.is_file()}
(dist/'source-binding.json').write_text(json.dumps(binding,indent=2)+'\n')
print('verified hooks',len(hooks),'native entry guards',len(rvas))
