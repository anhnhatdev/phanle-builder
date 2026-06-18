const png2icons = require('png2icons');
const fs = require('fs');
const path = require('path');

const inputPath = path.resolve(__dirname, '../resources/icons/icon.png');
const icoPath = path.resolve(__dirname, '../resources/icons/icon.ico');
const icnsPath = path.resolve(__dirname, '../resources/icons/icon.icns');

if (!fs.existsSync(inputPath)) {
  console.error('icon.png not found at:', inputPath);
  process.exit(1);
}

const inputBuffer = fs.readFileSync(inputPath);

console.log('Generating icon.ico...');
const icoBuffer = png2icons.createICO(inputBuffer, png2icons.HERMITE, 0);
if (icoBuffer) {
  fs.writeFileSync(icoPath, icoBuffer);
  console.log('✅ Generated icon.ico');
} else {
  console.error('Failed to generate icon.ico');
}

console.log('Generating icon.icns...');
const icnsBuffer = png2icons.createICNS(inputBuffer, png2icons.HERMITE, 0);
if (icnsBuffer) {
  fs.writeFileSync(icnsPath, icnsBuffer);
  console.log('✅ Generated icon.icns');
} else {
  console.error('Failed to generate icon.icns');
}
