package org.demo;

import cn.hutool.core.util.ZipUtil;
import org.apache.commons.io.IOUtils;

import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardCopyOption;
import java.util.ArrayList;
import java.util.List;
import java.util.zip.ZipEntry;
import java.util.zip.ZipInputStream;

public class Unzip {

    public static String readFileString(String zipPath, String fileName) throws IOException {
        try (InputStream inputStream = ZipUtil.get(new File(zipPath), StandardCharsets.UTF_8, fileName)) {
            if (null != inputStream) {
                return IOUtils.toString(inputStream, StandardCharsets.UTF_8);
            }
        }
        return null;
    }

    private static Path zipSlipProtect(ZipEntry zipEntry, Path targetDir) throws IOException {
        Path targetDirResolved = targetDir.resolve(zipEntry.getName());

        Path normalizePath = targetDirResolved.normalize();
        if (!normalizePath.startsWith(targetDir)) {
            throw new IOException("Bad zip entry: " + zipEntry.getName());
        }
        return normalizePath;
    }

    public static List<String> extract(String zipPath, String target, UnzipCallback callback) throws IOException {
        List<String> unzipFiles = new ArrayList<>();
        Path targetPath = Paths.get(target);
        try (FileInputStream fis = new FileInputStream(zipPath);
            ZipInputStream zis = new ZipInputStream(fis)) {
            ZipEntry zipEntry = zis.getNextEntry();

            while (zipEntry != null) {
                boolean isDirectory = zipEntry.getName().endsWith("/");

                Path newPath = zipSlipProtect(zipEntry, targetPath);
                if (isDirectory) {
                    Files.createDirectories(newPath);
                } else {
                    if (newPath.getParent() != null) {
                        if (Files.notExists(newPath.getParent())) {
                            Files.createDirectories(newPath.getParent());
                        }
                    }
                    Files.copy(zis, newPath, StandardCopyOption.REPLACE_EXISTING);
                }
                if (callback != null) {
                    callback.extractAfter(zipEntry, newPath);
                }
                unzipFiles.add(newPath.toString());
                zipEntry = zis.getNextEntry();
            }
            zis.closeEntry();
            return unzipFiles;
        } catch (IOException e) {
            throw e;
        }
    }
}
