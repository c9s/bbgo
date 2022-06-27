import Document, {DocumentContext, Head, Html, Main, NextScript} from 'next/document';

// ----- mantine setup
import {createStylesServer, ServerStyles} from '@mantine/next';
import {DocumentInitialProps} from "next/dist/shared/lib/utils";

// const getInitialProps = createGetInitialProps();
const stylesServer = createStylesServer();
// -----

class MyDocument extends Document {
  // this is for mantine
  // static getInitialProps = getInitialProps;

  static async getInitialProps(ctx: DocumentContext): Promise<DocumentInitialProps> {
    const initialProps = await Document.getInitialProps(ctx);

    return {
      ...initialProps,

      // use bracket [] instead of () to fix the type error
      styles: [
        <>
          {initialProps.styles}
          <ServerStyles key="server-styles" html={initialProps.html} server={stylesServer}/>
        </>
      ],
    };
  }

  render() {
    return (
      <Html lang="en">
        <Head> </Head>
        <body>
          <Main/>
          <NextScript/>
        </body>
      </Html>
    );
  }
}

export default MyDocument;
