// Listing printers and sending raw bytes to one. Windows is the real target (the kasa); macOS only
// exists so the agent can be developed here.
import { execFile } from 'node:child_process';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { promisify } from 'node:util';

const run = promisify(execFile);

export interface PrinterInfo { name: string; port: string | null; driver: string | null; isDefault: boolean }

export async function listPrinters(): Promise<PrinterInfo[]> {
  if (process.platform === 'win32') {
    const script = '$d=(Get-CimInstance Win32_Printer | Where-Object Default).Name; '
      + 'Get-Printer | ForEach-Object { [pscustomobject]@{ name=$_.Name; port=$_.PortName; driver=$_.DriverName; isDefault=($_.Name -eq $d) } } | ConvertTo-Json -Compress';
    const { stdout } = await run('powershell', ['-NoProfile', '-Command', script]);
    if (!stdout.trim()) return [];
    const parsed = JSON.parse(stdout) as PrinterInfo | PrinterInfo[]; // one printer comes back as an object, not an array
    return Array.isArray(parsed) ? parsed : [parsed];
  }
  const { stdout } = await run('lpstat', ['-e']).catch(() => ({ stdout: '' }));
  const def = (await run('lpstat', ['-d']).catch(() => ({ stdout: '' }))).stdout.split(':')[1]?.trim();
  return stdout.split('\n').map(s => s.trim()).filter(Boolean).map(name => ({ name, port: null, driver: null, isDefault: name === def }));
}

// Sends the bytes straight to the spooler as RAW (no driver rendering), like a thermal printer wants.
const WIN_RAW_SCRIPT = `
param([string]$Printer, [string]$File)
Add-Type -TypeDefinition @"
using System; using System.Runtime.InteropServices;
public class RawPrinter {
  [StructLayout(LayoutKind.Sequential, CharSet=CharSet.Unicode)]
  public class DOCINFO { public string pDocName; public string pOutputFile; public string pDataType; }
  [DllImport("winspool.drv", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool OpenPrinter(string n, out IntPtr h, IntPtr d);
  [DllImport("winspool.drv", SetLastError=true)] public static extern bool ClosePrinter(IntPtr h);
  [DllImport("winspool.drv", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool StartDocPrinter(IntPtr h, int l, [In] DOCINFO di);
  [DllImport("winspool.drv", SetLastError=true)] public static extern bool EndDocPrinter(IntPtr h);
  [DllImport("winspool.drv", SetLastError=true)] public static extern bool StartPagePrinter(IntPtr h);
  [DllImport("winspool.drv", SetLastError=true)] public static extern bool EndPagePrinter(IntPtr h);
  [DllImport("winspool.drv", SetLastError=true)] public static extern bool WritePrinter(IntPtr h, byte[] b, int c, out int w);
  public static void Send(string name, byte[] data) {
    IntPtr h;
    if (!OpenPrinter(name, out h, IntPtr.Zero)) throw new Exception("OpenPrinter failed: " + Marshal.GetLastWin32Error());
    try {
      var di = new DOCINFO { pDocName = "Mutfak On", pDataType = "RAW" };
      if (!StartDocPrinter(h, 1, di)) throw new Exception("StartDocPrinter failed: " + Marshal.GetLastWin32Error());
      StartPagePrinter(h);
      int written;
      if (!WritePrinter(h, data, data.Length, out written)) throw new Exception("WritePrinter failed: " + Marshal.GetLastWin32Error());
      EndPagePrinter(h); EndDocPrinter(h);
    } finally { ClosePrinter(h); }
  }
}
"@
[RawPrinter]::Send($Printer, [System.IO.File]::ReadAllBytes($File))
`;

export async function printRaw(printer: string, data: Buffer): Promise<void> {
  const dir = await mkdtemp(join(tmpdir(), 'mutfakon-print-'));
  try {
    const bin = join(dir, 'job.bin');
    await writeFile(bin, data);
    if (process.platform === 'win32') {
      const ps1 = join(dir, 'send.ps1');
      await writeFile(ps1, WIN_RAW_SCRIPT);
      await run('powershell', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', ps1, '-Printer', printer, '-File', bin]);
    } else {
      await run('lp', ['-d', printer, '-o', 'raw', bin]);
    }
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
}
