package org.demo;

import cn.hutool.core.io.IORuntimeException;
import org.jetbrains.annotations.NotNull;

import java.io.IOException;
import java.nio.file.Path;
import java.util.zip.ZipEntry;

//TIP To <b>Run</b> code, press <shortcut actionId="Run"/> or
// click the <icon src="AllIcons.Actions.Execute"/> icon in the gutter.
public class Main {
    public static void main(String @NotNull [] args) {
        if (args.length < 2) {
            System.out.println("Usage: java UnZip <source> <destination> [preview-file]");
            return;
        }

        String zipFilePath = args[0];
        String outputDir = args[1];
        String previewFile = args.length > 2 ? args[2] : null;

        System.out.printf("Unzipped %s to %s ... \n", zipFilePath, outputDir);

        try {
            if (previewFile != null && !previewFile.isEmpty()) {
                String preview = Unzip.readFileString(zipFilePath, previewFile);
                System.out.printf("* preview %s:\n", previewFile);
                System.out.println(preview);
            }
            Unzip.extract(zipFilePath, outputDir, new UnzipCallback() {
                @Override
                public void extractAfter(ZipEntry ze, Path path) {
                    System.out.printf("Extract %s to %s ... \n", ze.getName(), path);
                }
            });
        } catch (IOException e) {
            throw new IORuntimeException(e);
        }
    }
}