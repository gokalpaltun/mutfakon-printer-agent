// The few ESC/POS commands the agent needs itself. Receipt content comes from the app as a bitmap later.
const ESC = 0x1b, GS = 0x1d;

export function testReceipt(printerName: string): Buffer {
  const lines = ['MUTFAK ON', 'Test fisi / Test print', printerName, new Date().toLocaleString('tr-TR')];
  return Buffer.concat([
    Buffer.from([ESC, 0x40]),            // initialize
    Buffer.from([ESC, 0x61, 1]),         // center
    Buffer.from(lines.map(l => asciiOnly(l)).join('\n') + '\n', 'latin1'),
    Buffer.from('\n\n\n\n'),
    Buffer.from([GS, 0x56, 66, 0]),      // feed and partial cut
  ]);
}

// Test text stays ASCII so it prints whatever code page the printer is on.
function asciiOnly(s: string): string {
  const map: Record<string, string> = { ç: 'c', Ç: 'C', ğ: 'g', Ğ: 'G', ı: 'i', İ: 'I', ö: 'o', Ö: 'O', ş: 's', Ş: 'S', ü: 'u', Ü: 'U' };
  return s.replace(/[^\x00-\x7f]/g, c => map[c] ?? '?');
}
