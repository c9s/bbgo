import Document, {DocumentContext, Head, Html, Main, NextScript} from 'next/document';
import {CssBaseline} from '@nextui-org/react';

class MyDocument extends Document {
  static async getInitialProps(ctx: DocumentContext) {
    const initialProps = await Document.getInitialProps(ctx);

    // @ts-ignore
    initialProps.styles = <>{initialProps.styles}</>;
    return initialProps;
  }

  render() {
    return (
      <Html lang="en">
        <Head>
          {CssBaseline.flush()}
        </Head>
        <body>
          <Main/>
          <NextScript/>
        </body>
      </Html>
    );
  }
}

export default MyDocument;
