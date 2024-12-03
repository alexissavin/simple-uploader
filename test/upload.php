<?php

function data_collection_send($url)
{

  $ts = microtime(TRUE);
  $files = glob('./*.tgz');

  if (is_array($files) && count($files)) {
    $ch = curl_init();
    foreach ($files as $f) {
      echo('Data Collection: Sending file ['.$f."]\n");
      if (empty(($file = curl_file_create($f)))) {
        echo('Data Collection: Failed to create curl file ['.$f."]\n");
        continue;
      }

      curl_setopt($ch, CURLOPT_URL, $url);
      curl_setopt($ch, CURLOPT_HEADER, 0);
      curl_setopt($ch, CURLOPT_POST, TRUE);
      curl_setopt($ch, CURLOPT_POSTFIELDS, ['file' => $file]);
      curl_setopt($ch, CURLOPT_RETURNTRANSFER, TRUE);
      curl_setopt($ch, CURLOPT_CONNECTTIMEOUT, 30);
      curl_setopt($ch, CURLOPT_TIMEOUT, 30);
      curl_setopt($ch, CURLOPT_DNS_SHUFFLE_ADDRESSES, TRUE);
      curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, TRUE);
      curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, 2);
      curl_setopt($ch, CURLOPT_ENCODING, 'gzip,deflate');

      if (!empty($server['proxy']))
        echo("Data Collection: Using proxy [127.0.0.1:8888]\n");
        curl_setopt($ch, CURLOPT_PROXY, "127.0.0.1:8888");

      if (($res = curl_exec($ch)) === FALSE) {
        echo('Data Collection: Failed to upload file ['.$f.'] ('.curl_errno($ch).'): '.curl_error($ch)."\n");
        curl_close($ch);
        return (FALSE);
      }

      $nfo = curl_getinfo($ch);

      if ($nfo['http_code'] != 200) {
        echo('Data Collection: Failed to upload file ['.$f.']: HTTP ('.$nfo['http_code'].")\n");
        curl_close($ch);
        return (FALSE);
      }

      //{"ok":true,"path":"/files/<token>/data-collection_<serial>_<timestamp>.tgz"}
      if (empty($res) || empty(($json = json_decode($res))) || empty($json->ok) || empty($json->path)) {
        echo('Data Collection: failed to upload file ['.$f.']: '.$res);
      }
    }

    curl_close($ch);
  }

  echo("Data Collection: Completed\n");
}

if ($argc == 2) {
  echo("Data Collection: Target URL [" . $argv[1] . "]\n");
  data_collection_send($argv[1]);
} else {
  echo("Please specify some URL and token such as: https://upload.test.com/upload?token=02c8797e-981e-11ee-82f6-674e065a753c\n");
}