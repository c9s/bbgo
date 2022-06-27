import '../styles/globals.css'
import '../components/TimeRangeSlider/index.scss'

import type {AppProps} from 'next/app'
import Head from 'next/head';

import { MantineProvider } from '@mantine/core';

function MyApp({Component, pageProps}: AppProps) {
  return <>
    <Head>
      <title>BBGO Back-test Report</title>
      <meta name="viewport" content="minimum-scale=1, initial-scale=1, width=device-width" />
    </Head>
    <MantineProvider
      withGlobalStyles
      withNormalizeCSS
      theme={{
        /** Put your mantine theme override here */
        colorScheme: 'light',
      }}
    >
      <Component {...pageProps} />
    </MantineProvider>
  </>
}

export default MyApp
