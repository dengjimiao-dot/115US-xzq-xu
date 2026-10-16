from pathlib import Path
import sys,hashlib,json
root=Path(__file__).resolve().parent

import pefile,capstone
exe=Path(sys.argv[1]) if len(sys.argv)>1 else sys.exit('Usage: python generate-evidence.py PATH_TO_DFO_EXE')
sha=hashlib.sha256(exe.read_bytes()).hexdigest()
assert sha=='515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb'
pe=pefile.PE(str(exe),fast_load=True)
md=capstone.Cs(capstone.CS_ARCH_X86,capstone.CS_MODE_64)
spec=[('Menu',0x1a04880,9),('State',0x59a90f0,16),('Dungeon',0x59a9080,16),('Player',0x5efafb0,16)]
out=['#pragma once','#include <stdint.h>',f'static const DWORD kImageTimestamp={pe.FILE_HEADER.TimeDateStamp}u;',f'static const DWORD kImageSize={pe.OPTIONAL_HEADER.SizeOfImage}u;']
binding={'client_sha256':sha,'functions':{},'menu_actions':[22,23,30],'live_writes':0}
for name,rva,size in spec:
    data=pe.get_data(rva,size)
    if name=='Menu':
        instructions=list(md.disasm(data,pe.OPTIONAL_HEADER.ImageBase+rva))
        assert sum(i.size for i in instructions)==size
        assert [(i.mnemonic,i.op_str) for i in instructions]==[('push','rsi'),('push','rdi'),('push','r14'),('sub','rsp, 0x30')]
    out.extend([f'static const uintptr_t k{name}RVA={hex(rva)};',f'static const unsigned char k{name}Head[]={{'+','.join(hex(b) for b in data)+'};'])
    binding['functions'][name]={'rva':hex(rva),'bytes':data.hex()}
(root/'dist').mkdir(exist_ok=True)
(root/'dist/native_evidence.h').write_text('\n'.join(out)+'\n',encoding='utf-8')
(root/'dist/source-binding.json').write_text(json.dumps(binding,indent=2)+'\n',encoding='utf-8')
print(json.dumps(binding))
