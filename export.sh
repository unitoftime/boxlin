#!/bin/bash

# TODO - Convert to go file? generator.go? https://stackoverflow.com/questions/55598931/go-generate-multiline-command

set -x # Activate debugging

echo "Exporting Aseprite Files to ase/images"
mkdir -p tmp

# Exporting with tags and trimming
filenames=(package peg wall packing-line)
for file in ${filenames[@]}
do
    aseprite -b ./${file}.ase --save-as "./tmp/${file}-{frame}.png"
    mogrify -trim tmp/${file}-*.png
done


# Pack all images into a spritesheet
packer --input ./tmp --stats --output assets/spritesheet

#mkdir -p assets/fonts
#cp ase/fonts/* assets/fonts/

# Remove generated images
rm -f ./tmp/*
#rm -f ase/mount/*
