$dirs = @(
   "cmd\api",
   "internal\audio",
   "internal\dsp",
   "internal\store",
   "internal\matcher",
   "internal\api\handlers",
   "internal\api\dto",
   "internal\api\middleware",
   "internal\config",
   "internal\models",
    "db\migrations",
    "db\seeds",
    "tests\integration",
    "tests\testdata"
)

foreach ($d in $dirs) {
   New-Item -ItemType Directory -Path $d -Force
   Write-Host "Created: $d"
}